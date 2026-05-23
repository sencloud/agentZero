import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';

import '../../models/mission.dart';

/// MissionEventStream 把后端 `GET /missions/:id/stream` 的 SSE 解码成
/// [MissionStep] 流。
///
/// 后端事件协议（每条事件由几行字段组成）：
///   `event: step`
///   `id: SEQ`
///   `data: JSON`
///
/// 也会偶尔收到 `event: closed`（mission 终态）或心跳 `: keepalive` 行。
class MissionEventStream {
  MissionEventStream(this._dio);
  final Dio _dio;

  /// 订阅一个 mission 的事件流，返回的 Stream 在连接关闭时自然结束。
  /// 调用方一般通过 StreamSubscription.cancel() 来主动断开。
  ///
  /// [afterSeq] >0 时，服务端会在订阅前先把 seq>afterSeq 的历史事件回放过来。
  Stream<MissionStep> connect(String missionId, {int afterSeq = 0}) async* {
    final res = await _dio.get<ResponseBody>(
      '/missions/$missionId/stream',
      queryParameters: {if (afterSeq > 0) 'after_seq': afterSeq},
      options: Options(
        responseType: ResponseType.stream,
        headers: {'Accept': 'text/event-stream'},
        // 长连接：不要让 dio 的接收超时把流给掐了
        receiveTimeout: Duration.zero,
      ),
    );
    final body = res.data;
    if (body == null) return;

    // ResponseBody.stream 是 Stream<Uint8List>；要 cast 成 Stream<List<int>>
    // 才能喂给 utf8.decoder。
    final lines = body.stream.cast<List<int>>().transform(utf8.decoder).transform(const LineSplitter());

    String? currentEvent;
    final dataBuf = StringBuffer();

    await for (final line in lines) {
      if (line.isEmpty) {
        // SSE 一条事件结束
        if (currentEvent == 'step' && dataBuf.isNotEmpty) {
          try {
            final json = jsonDecode(dataBuf.toString()) as Map<String, dynamic>;
            yield MissionStep.fromJson(json);
          } catch (_) {
            // 单条解析失败不致命，继续下一条
          }
        }
        currentEvent = null;
        dataBuf.clear();
        continue;
      }
      if (line.startsWith(':')) {
        // 心跳/注释行
        continue;
      }
      if (line.startsWith('event:')) {
        currentEvent = line.substring(6).trim();
      } else if (line.startsWith('data:')) {
        if (dataBuf.isNotEmpty) dataBuf.write('\n');
        dataBuf.write(line.substring(5).trim());
      }
      // 其他字段（id: / retry:）当前忽略
    }
  }
}

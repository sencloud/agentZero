import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../core/theme.dart';
import '../../providers/feed.dart';

/// 刷新事件流的实时进度面板。
///
/// 数据流：
///   - Provider 暴露一个 `StateNotifier<FeedRefreshState>`，State 含 progress events
///   - 用户点击 FeedPage 的刷新按钮时，调 notifier.start()
///   - notifier 通过 dio 拿 ResponseBody.stream，逐行解析 `data: <json>`，append 到 events
///   - 完成后保留最近一次记录直到下次重启
class FeedRefreshEvent {
  FeedRefreshEvent({
    required this.phase,
    required this.message,
    this.data = const {},
    DateTime? at,
  }) : at = at ?? DateTime.now();

  final String phase;
  final String message;
  final Map<String, dynamic> data;
  final DateTime at;
}

class FeedRefreshState {
  const FeedRefreshState({
    required this.running,
    required this.events,
    this.error,
  });

  final bool running;
  final List<FeedRefreshEvent> events;
  final String? error;

  static const idle = FeedRefreshState(running: false, events: []);

  FeedRefreshState copyWith({bool? running, List<FeedRefreshEvent>? events, String? error}) =>
      FeedRefreshState(
        running: running ?? this.running,
        events: events ?? this.events,
        error: error,
      );
}

class FeedRefreshNotifier extends StateNotifier<FeedRefreshState> {
  FeedRefreshNotifier(this._ref) : super(FeedRefreshState.idle);

  final Ref _ref;
  StreamSubscription<List<int>>? _sub;

  Future<void> start() async {
    if (state.running) return;
    state = const FeedRefreshState(running: true, events: []);
    final dio = _ref.read(apiClientProvider).dio;
    try {
      final res = await dio.get<ResponseBody>(
        '/feed/refresh/stream',
        options: Options(
          responseType: ResponseType.stream,
          headers: {'Accept': 'text/event-stream'},
          receiveTimeout: Duration.zero,
        ),
      );
      final body = res.data;
      if (body == null) {
        state = state.copyWith(running: false, error: '空响应');
        return;
      }
      final buffer = StringBuffer();
      final completer = Completer<void>();
      _sub = body.stream.cast<List<int>>().listen(
        (chunk) {
          buffer.write(utf8.decode(chunk, allowMalformed: true));
          final text = buffer.toString();
          // 按 SSE 单事件分隔符（两个换行）切片
          final parts = text.split('\n\n');
          // 最后一片可能没收满，保留到下一次拼接
          for (var i = 0; i < parts.length - 1; i++) {
            final raw = parts[i].trim();
            if (raw.isEmpty || !raw.startsWith('data:')) continue;
            final payload = raw.substring(5).trim();
            try {
              final j = Map<String, dynamic>.from(jsonDecode(payload));
              final ev = FeedRefreshEvent(
                phase: (j['phase'] as String?) ?? 'unknown',
                message: (j['message'] as String?) ?? '',
                data: Map<String, dynamic>.from(j['data'] as Map? ?? const {}),
              );
              final updated = [...state.events, ev];
              state = state.copyWith(events: updated);
              if (ev.phase == 'done' || ev.phase == 'error') {
                // 完成后刷新依赖数据
                _ref.invalidate(feedStatusProvider);
                _ref.invalidate(feedEventsProvider);
                _ref.invalidate(feedGraphProvider);
              }
            } catch (_) {
              // 单条解析失败不打断整流
            }
          }
          buffer
            ..clear()
            ..write(parts.last);
        },
        onDone: () {
          state = state.copyWith(running: false);
          if (!completer.isCompleted) completer.complete();
        },
        onError: (e) {
          state = state.copyWith(running: false, error: e.toString());
          if (!completer.isCompleted) completer.complete();
        },
        cancelOnError: true,
      );
      await completer.future;
    } catch (e) {
      state = state.copyWith(running: false, error: e.toString());
    }
  }

  void clear() {
    state = FeedRefreshState.idle;
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }
}

final feedRefreshProvider =
    StateNotifierProvider<FeedRefreshNotifier, FeedRefreshState>((ref) => FeedRefreshNotifier(ref));

/// 进度面板：折叠在顶部，运行中和完成后展示 timeline。
class FeedRefreshPanel extends ConsumerWidget {
  const FeedRefreshPanel({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final st = ref.watch(feedRefreshProvider);
    if (st.events.isEmpty && !st.running && st.error == null) {
      return const SizedBox.shrink();
    }
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 8, 12, 8),
      decoration: BoxDecoration(
        color: AppTheme.carbon,
        border: Border.all(color: AppTheme.graphite, width: 0.8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _PanelHeader(state: st, onClose: () => ref.read(feedRefreshProvider.notifier).clear()),
          if (st.events.isNotEmpty)
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 200),
              child: ListView.builder(
                shrinkWrap: true,
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                itemCount: st.events.length,
                itemBuilder: (_, i) => _EventRow(event: st.events[i]),
              ),
            ),
        ],
      ),
    );
  }
}

class _PanelHeader extends StatelessWidget {
  const _PanelHeader({required this.state, required this.onClose});
  final FeedRefreshState state;
  final VoidCallback onClose;

  @override
  Widget build(BuildContext context) {
    final running = state.running;
    final tag = running ? 'REFRESHING' : (state.error != null ? 'FAILED' : 'DONE');
    final tagColor = running
        ? const Color(0xFF67D17B)
        : (state.error != null ? AppTheme.redline : AppTheme.amber);
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 8, 8, 8),
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: AppTheme.graphite, width: 0.6)),
      ),
      child: Row(
        children: [
          if (running)
            const SizedBox(
              width: 12,
              height: 12,
              child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTheme.paper),
            )
          else
            Icon(state.error != null ? CupertinoIcons.exclamationmark : CupertinoIcons.check_mark,
                size: 14, color: tagColor),
          const SizedBox(width: 8),
          Text(
            tag,
            style: TextStyle(
              color: tagColor,
              fontSize: 10,
              letterSpacing: 3,
              fontWeight: FontWeight.w700,
              fontFamilyFallback: AppTheme.monoFallback,
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              running ? '刷新进行中…' : (state.error ?? '刷新结束'),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: AppTheme.pen, fontSize: 11, letterSpacing: 1),
            ),
          ),
          if (!running)
            IconButton(
              icon: const Icon(CupertinoIcons.xmark, size: 14, color: AppTheme.muted),
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
              onPressed: onClose,
            ),
        ],
      ),
    );
  }
}

class _EventRow extends StatelessWidget {
  const _EventRow({required this.event});
  final FeedRefreshEvent event;

  @override
  Widget build(BuildContext context) {
    final tag = _shortTag(event.phase);
    final color = _phaseColor(event.phase);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 64,
            padding: const EdgeInsets.symmetric(vertical: 2, horizontal: 4),
            alignment: Alignment.center,
            decoration: BoxDecoration(border: Border.all(color: color, width: 0.6)),
            child: Text(
              tag,
              style: TextStyle(
                color: color,
                fontSize: 9,
                letterSpacing: 1.5,
                fontFamilyFallback: AppTheme.monoFallback,
              ),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              event.message,
              style: const TextStyle(color: AppTheme.paper, fontSize: 11.5, height: 1.5),
            ),
          ),
          const SizedBox(width: 6),
          Text(
            _hms(event.at),
            style: const TextStyle(
              color: AppTheme.muted,
              fontSize: 10,
              fontFamilyFallback: AppTheme.monoFallback,
            ),
          ),
        ],
      ),
    );
  }

  static String _shortTag(String phase) {
    switch (phase) {
      case 'start':
        return 'START';
      case 'recommend_start':
        return 'PICK';
      case 'recommend_done':
        return 'PICKED';
      case 'recommend_error':
        return 'PICK✗';
      case 'fetch_start':
        return 'FETCH';
      case 'fetch_source':
        return 'SRC';
      case 'fetch_done':
        return 'FETCHED';
      case 'match_start':
      case 'match_done':
        return 'MATCH';
      case 'extract_start':
      case 'extract_done':
        return 'EXTRACT';
      case 'extract_event':
        return 'LLM';
      case 'done':
        return 'DONE';
      case 'error':
        return 'ERR';
      default:
        return phase.toUpperCase();
    }
  }

  static Color _phaseColor(String phase) {
    switch (phase) {
      case 'fetch_source':
      case 'extract_event':
        return AppTheme.paper;
      case 'recommend_done':
      case 'fetch_done':
      case 'extract_done':
      case 'match_done':
      case 'done':
        return AppTheme.amber;
      case 'recommend_error':
      case 'error':
        return AppTheme.redline;
      default:
        return AppTheme.pen;
    }
  }

  static String _hms(DateTime t) {
    String pad(int x) => x.toString().padLeft(2, '0');
    return '${pad(t.hour)}:${pad(t.minute)}:${pad(t.second)}';
  }
}

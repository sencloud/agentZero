import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../models/mission.dart';

/// 拉登录用户的任务列表（pending/running 在前，归档在后，由后端按 created_at 排序）。
final missionsListProvider = FutureProvider.autoDispose<List<Mission>>((ref) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get<Map<String, dynamic>>('/missions');
  final items = (r.data?['items'] as List?) ?? const [];
  return items.map((e) => Mission.fromJson(e as Map<String, dynamic>)).toList();
});

/// 列出后端注册的所有装备（用于派遣页勾选）。
final toolsProvider = FutureProvider.autoDispose<List<ToolInfo>>((ref) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get<Map<String, dynamic>>('/tools');
  final items = (r.data?['items'] as List?) ?? const [];
  return items.map((e) => ToolInfo.fromJson(e as Map<String, dynamic>)).toList();
});

/// 派遣请求结果。
class DispatchedMission {
  DispatchedMission(this.mission);
  final Mission mission;
}

/// 派遣单个任务。返回新创建的 Mission（已 running）。
class DispatchMissionAction {
  DispatchMissionAction(this.ref);
  final Ref ref;

  Future<Mission> call({
    required String codename,
    required String brief,
    required MissionTier tier,
    required List<String> loadout,
  }) async {
    final api = ref.read(apiClientProvider);
    final r = await api.dio.post<Map<String, dynamic>>('/missions', data: {
      'codename': codename,
      'brief': brief,
      'tier': tier.wire,
      'loadout': loadout,
    });
    final m = Mission.fromJson(r.data!['mission'] as Map<String, dynamic>);
    ref.invalidate(missionsListProvider);
    return m;
  }
}

final dispatchMissionProvider = Provider<DispatchMissionAction>((ref) => DispatchMissionAction(ref));

/// 任务详情 + 历史 steps + artifacts。
class MissionDetail {
  MissionDetail({required this.mission, required this.steps, required this.artifacts, required this.running});
  final Mission mission;
  final List<MissionStep> steps;
  final List<Artifact> artifacts;
  final bool running;
}

final missionDetailProvider = FutureProvider.autoDispose.family<MissionDetail, String>((ref, id) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get<Map<String, dynamic>>('/missions/$id');
  final data = r.data!;
  final mission = Mission.fromJson(data['mission'] as Map<String, dynamic>);
  final steps = ((data['steps'] as List?) ?? const [])
      .map((e) => MissionStep.fromJson(e as Map<String, dynamic>))
      .toList();
  final artifacts = ((data['artifacts'] as List?) ?? const [])
      .map((e) => Artifact.fromJson(e as Map<String, dynamic>))
      .toList();
  return MissionDetail(
    mission: mission,
    steps: steps,
    artifacts: artifacts,
    running: (data['running'] as bool?) ?? false,
  );
});

/// 撤离（abort）正在运行的任务。
class AbortMissionAction {
  AbortMissionAction(this.ref);
  final Ref ref;
  Future<void> call(String id) async {
    final api = ref.read(apiClientProvider);
    await api.dio.post('/missions/$id/abort');
  }
}

final abortMissionProvider = Provider<AbortMissionAction>((ref) => AbortMissionAction(ref));

/// 工件原始内容（不缓存，用于 ArtifactViewerPage 拉取 HTML 等）。
class ArtifactContent {
  ArtifactContent({required this.bytes, required this.mime, required this.text});
  final List<int> bytes;
  final String mime;
  final String text;

  bool get isHtml => mime.startsWith('text/html');
  bool get isText => mime.startsWith('text/') || mime == 'application/json';
}

class _ArtifactKey {
  _ArtifactKey(this.missionId, this.artifactId);
  final String missionId;
  final int artifactId;

  @override
  bool operator ==(Object other) =>
      other is _ArtifactKey && other.missionId == missionId && other.artifactId == artifactId;
  @override
  int get hashCode => Object.hash(missionId, artifactId);
}

final artifactContentProvider =
    FutureProvider.autoDispose.family<ArtifactContent, _ArtifactKey>((ref, key) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get<List<int>>(
    '/missions/${key.missionId}/artifacts/${key.artifactId}/content',
    options: Options(responseType: ResponseType.bytes),
  );
  final mime = (r.headers.value('content-type') ?? '').split(';').first.trim();
  final bytes = r.data ?? const <int>[];
  String text = '';
  if (mime.startsWith('text/') || mime == 'application/json') {
    try {
      text = utf8.decode(bytes);
    } catch (_) {
      text = '';
    }
  }
  return ArtifactContent(bytes: bytes, mime: mime, text: text);
});

/// 给外部调用的便利函数：拉一个工件的内容。
Future<ArtifactContent> fetchArtifactContent(Ref ref, String missionId, int artifactId) {
  return ref.read(artifactContentProvider(_ArtifactKey(missionId, artifactId)).future);
}

/// 给 widget tree 内调用：直接 watch 工件内容。
AsyncValue<ArtifactContent> watchArtifactContent(WidgetRef ref, String missionId, int artifactId) {
  return ref.watch(artifactContentProvider(_ArtifactKey(missionId, artifactId)));
}

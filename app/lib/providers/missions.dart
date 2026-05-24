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

/// 销毁一个任务（包含 steps / artifacts / workspace）。
class DeleteMissionAction {
  DeleteMissionAction(this.ref);
  final Ref ref;
  Future<void> call(String id) async {
    final api = ref.read(apiClientProvider);
    await api.dio.delete('/missions/$id');
    ref.invalidate(missionsListProvider);
  }
}

final deleteMissionProvider = Provider<DeleteMissionAction>((ref) => DeleteMissionAction(ref));

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

// ============== 点评 / Skill / 卷宗 / 继续安排 ==============

/// 拉一个 mission 的当前点评（无评则 null）。
final missionReviewProvider =
    FutureProvider.autoDispose.family<MissionReview?, String>((ref, missionId) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get<Map<String, dynamic>>('/missions/$missionId/review');
  final v = r.data?['review'];
  if (v == null) return null;
  return MissionReview.fromJson(v as Map<String, dynamic>);
});

/// 提交/更新一次点评。
class SubmitReviewAction {
  SubmitReviewAction(this.ref);
  final Ref ref;
  Future<MissionReview> call({
    required String missionId,
    required int rating,
    required String comment,
  }) async {
    final api = ref.read(apiClientProvider);
    final r = await api.dio.post<Map<String, dynamic>>(
      '/missions/$missionId/review',
      data: {'rating': rating, 'comment': comment},
    );
    final rv = MissionReview.fromJson(r.data!['review'] as Map<String, dynamic>);
    ref.invalidate(missionReviewProvider(missionId));
    return rv;
  }
}

final submitReviewProvider = Provider<SubmitReviewAction>((ref) => SubmitReviewAction(ref));

/// 列出用户沉淀的 Skill。
final skillsListProvider = FutureProvider.autoDispose<List<Skill>>((ref) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get<Map<String, dynamic>>('/skills');
  final items = (r.data?['items'] as List?) ?? const [];
  return items.map((e) => Skill.fromJson(e as Map<String, dynamic>)).toList();
});

/// 新建一项 Skill。
class CreateSkillAction {
  CreateSkillAction(this.ref);
  final Ref ref;
  Future<Skill> call({
    required String name,
    required String description,
    required String triggerHint,
    required String promptTemplate,
    String? sourceMissionId,
  }) async {
    final api = ref.read(apiClientProvider);
    final r = await api.dio.post<Map<String, dynamic>>('/skills', data: {
      'name': name,
      'description': description,
      'trigger_hint': triggerHint,
      'prompt_template': promptTemplate,
      'source_mission_id': ?sourceMissionId,
    });
    ref.invalidate(skillsListProvider);
    return Skill.fromJson(r.data!['skill'] as Map<String, dynamic>);
  }
}

final createSkillProvider = Provider<CreateSkillAction>((ref) => CreateSkillAction(ref));

/// 拉同卷宗下的全部 mission。
final missionSeriesProvider =
    FutureProvider.autoDispose.family<List<Mission>, String>((ref, missionId) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get<Map<String, dynamic>>('/missions/$missionId/series');
  final items = (r.data?['items'] as List?) ?? const [];
  return items.map((e) => Mission.fromJson(e as Map<String, dynamic>)).toList();
});

/// 「继续安排」：在指定 mission 后面追派一份新任务。
class FollowUpMissionAction {
  FollowUpMissionAction(this.ref);
  final Ref ref;
  Future<Mission> call({
    required String parentId,
    required String codename,
    required String brief,
    MissionTier? tier,
    List<String>? loadout,
  }) async {
    final api = ref.read(apiClientProvider);
    final r = await api.dio.post<Map<String, dynamic>>(
      '/missions/$parentId/follow_up',
      data: {
        'codename': codename,
        'brief': brief,
        'tier': ?tier?.wire,
        'loadout': ?loadout,
      },
    );
    final m = Mission.fromJson(r.data!['mission'] as Map<String, dynamic>);
    ref.invalidate(missionsListProvider);
    return m;
  }
}

final followUpMissionProvider = Provider<FollowUpMissionAction>((ref) => FollowUpMissionAction(ref));

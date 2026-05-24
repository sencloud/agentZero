// 与后端 internal/model/mission.go 的 JSON 形态一一对应。
// 任何字段含义/契约变更都需要后端同步。

enum MissionTier { flash, standard, pro }

extension MissionTierX on MissionTier {
  String get wire {
    switch (this) {
      case MissionTier.flash:
        return 'flash';
      case MissionTier.standard:
        return 'standard';
      case MissionTier.pro:
        return 'pro';
    }
  }

  String get label {
    switch (this) {
      case MissionTier.flash:
        return '闪电';
      case MissionTier.standard:
        return '标准';
      case MissionTier.pro:
        return '深度';
    }
  }

  String get desc {
    switch (this) {
      case MissionTier.flash:
        return '不思考 · 最快响应';
      case MissionTier.standard:
        return '默认思考 · 性价比最高';
      case MissionTier.pro:
        return '深度推理 · 适合复杂任务';
    }
  }

  static MissionTier fromWire(String s) =>
      MissionTier.values.firstWhere((t) => t.wire == s, orElse: () => MissionTier.standard);
}

enum MissionStatus { pending, running, done, aborted, error }

extension MissionStatusX on MissionStatus {
  String get wire => name;

  String get label {
    switch (this) {
      case MissionStatus.pending:
        return '待命';
      case MissionStatus.running:
        return '行动中';
      case MissionStatus.done:
        return '已完成';
      case MissionStatus.aborted:
        return '已撤离';
      case MissionStatus.error:
        return '失联';
    }
  }

  static MissionStatus fromWire(String s) =>
      MissionStatus.values.firstWhere((t) => t.name == s, orElse: () => MissionStatus.pending);

  bool get isTerminal => this == MissionStatus.done || this == MissionStatus.aborted || this == MissionStatus.error;
}

class Mission {
  Mission({
    required this.id,
    required this.codename,
    required this.brief,
    required this.tier,
    required this.status,
    required this.loadout,
    required this.inputTokens,
    required this.outputTokens,
    required this.createdAt,
    required this.seriesId,
    required this.seriesSeq,
    this.parentId,
    this.startedAt,
    this.endedAt,
  });

  final String id;
  final String codename;
  final String brief;
  final MissionTier tier;
  final MissionStatus status;
  final List<String> loadout;
  final int inputTokens;
  final int outputTokens;
  final DateTime createdAt;
  final String seriesId;
  final int seriesSeq;
  final String? parentId;
  final DateTime? startedAt;
  final DateTime? endedAt;

  factory Mission.fromJson(Map<String, dynamic> json) => Mission(
        id: json['id'] as String,
        codename: (json['codename'] as String?) ?? '未命名行动',
        brief: (json['brief'] as String?) ?? '',
        tier: MissionTierX.fromWire((json['tier'] as String?) ?? 'standard'),
        status: MissionStatusX.fromWire((json['status'] as String?) ?? 'pending'),
        loadout: ((json['loadout'] as List?) ?? const []).cast<String>(),
        inputTokens: (json['input_tokens'] as num?)?.toInt() ?? 0,
        outputTokens: (json['output_tokens'] as num?)?.toInt() ?? 0,
        createdAt: DateTime.parse(json['created_at'] as String).toLocal(),
        seriesId: (json['series_id'] as String?) ?? (json['id'] as String),
        seriesSeq: (json['series_seq'] as num?)?.toInt() ?? 1,
        parentId: json['parent_id'] as String?,
        startedAt: (json['started_at'] as String?) != null
            ? DateTime.parse(json['started_at'] as String).toLocal()
            : null,
        endedAt: (json['ended_at'] as String?) != null
            ? DateTime.parse(json['ended_at'] as String).toLocal()
            : null,
      );
}

/// 用户对一次行动的点评（1-5 星 + 评语）。
class MissionReview {
  MissionReview({
    required this.missionId,
    required this.rating,
    required this.comment,
    required this.createdAt,
    required this.updatedAt,
  });

  final String missionId;
  final int rating;
  final String comment;
  final DateTime createdAt;
  final DateTime updatedAt;

  factory MissionReview.fromJson(Map<String, dynamic> json) => MissionReview(
        missionId: json['mission_id'] as String,
        rating: (json['rating'] as num).toInt(),
        comment: (json['comment'] as String?) ?? '',
        createdAt: DateTime.parse(json['created_at'] as String).toLocal(),
        updatedAt: DateTime.parse(json['updated_at'] as String).toLocal(),
      );
}

/// 沉淀下来的技能（高分行动总结成的 Skill）。
class Skill {
  Skill({
    required this.id,
    required this.name,
    required this.description,
    required this.triggerHint,
    required this.promptTemplate,
    required this.createdAt,
    this.sourceMissionId,
  });

  final int id;
  final String name;
  final String description;
  final String triggerHint;
  final String promptTemplate;
  final String? sourceMissionId;
  final DateTime createdAt;

  factory Skill.fromJson(Map<String, dynamic> json) => Skill(
        id: (json['id'] as num).toInt(),
        name: (json['name'] as String?) ?? '',
        description: (json['description'] as String?) ?? '',
        triggerHint: (json['trigger_hint'] as String?) ?? '',
        promptTemplate: (json['prompt_template'] as String?) ?? '',
        sourceMissionId: json['source_mission_id'] as String?,
        createdAt: DateTime.parse(json['created_at'] as String).toLocal(),
      );
}

/// Step 是事件流上的单条原子事件。payload 解码时按 type 自行 cast。
class MissionStep {
  MissionStep({
    required this.id,
    required this.missionId,
    required this.seq,
    required this.ts,
    required this.type,
    required this.payload,
  });

  final int id;
  final String missionId;
  final int seq;
  final DateTime ts;
  final String type;
  final Map<String, dynamic> payload;

  factory MissionStep.fromJson(Map<String, dynamic> json) => MissionStep(
        id: (json['id'] as num).toInt(),
        missionId: json['mission_id'] as String,
        seq: (json['seq'] as num).toInt(),
        ts: DateTime.parse(json['ts'] as String).toLocal(),
        type: json['type'] as String,
        payload: (json['payload'] as Map?)?.cast<String, dynamic>() ?? const {},
      );
}

class Artifact {
  Artifact({
    required this.id,
    required this.missionId,
    required this.kind,
    required this.name,
    required this.path,
    required this.mime,
    required this.size,
    required this.createdAt,
  });

  final int id;
  final String missionId;
  final String kind;
  final String name;
  final String path;
  final String mime;
  final int size;
  final DateTime createdAt;

  factory Artifact.fromJson(Map<String, dynamic> json) => Artifact(
        id: (json['id'] as num).toInt(),
        missionId: json['mission_id'] as String,
        kind: (json['kind'] as String?) ?? 'file',
        name: (json['name'] as String?) ?? '',
        path: (json['path'] as String?) ?? '',
        mime: (json['mime'] as String?) ?? '',
        size: (json['size'] as num?)?.toInt() ?? 0,
        createdAt: DateTime.parse(json['created_at'] as String).toLocal(),
      );
}

class ToolInfo {
  ToolInfo({required this.name, required this.displayName, required this.description});
  final String name;
  final String displayName;
  final String description;

  factory ToolInfo.fromJson(Map<String, dynamic> json) => ToolInfo(
        name: json['name'] as String,
        displayName: (json['display_name'] as String?) ?? json['name'] as String,
        description: (json['description'] as String?) ?? '',
      );
}

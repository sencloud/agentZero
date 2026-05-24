/// 事件流图谱相关的数据模型。
///
/// 后端 JSON 都使用 snake_case，前端用 camelCase 保留 Dart 风格。
class FeedStatus {
  FeedStatus({
    required this.running,
    required this.sourcesTotal,
    required this.sourcesActive,
    required this.events24h,
    required this.entitiesTotal,
    required this.relationsTotal,
    this.lastFetchAt,
    this.lastPruneAt,
    this.lastError = '',
  });

  final bool running;
  final int sourcesTotal;
  final int sourcesActive;
  final int events24h;
  final int entitiesTotal;
  final int relationsTotal;
  final DateTime? lastFetchAt;
  final DateTime? lastPruneAt;
  final String lastError;

  bool get healthy => running && lastError.isEmpty;

  factory FeedStatus.fromJson(Map<String, dynamic> j) => FeedStatus(
        running: j['running'] == true,
        sourcesTotal: (j['sources_total'] ?? 0) as int,
        sourcesActive: (j['sources_active'] ?? 0) as int,
        events24h: (j['events_24h'] ?? 0) as int,
        entitiesTotal: (j['entities_total'] ?? 0) as int,
        relationsTotal: (j['relations_total'] ?? 0) as int,
        lastFetchAt: _parseTime(j['last_fetch_at']),
        lastPruneAt: _parseTime(j['last_prune_at']),
        lastError: (j['last_error'] as String?) ?? '',
      );
}

class Topic {
  Topic({
    required this.id,
    required this.name,
    required this.weight,
    required this.createdAt,
  });

  final int id;
  final String name;
  final double weight;
  final DateTime createdAt;

  factory Topic.fromJson(Map<String, dynamic> j) => Topic(
        id: (j['id'] as num).toInt(),
        name: j['name'] as String,
        weight: (j['weight'] as num?)?.toDouble() ?? 1.0,
        createdAt: _parseTime(j['created_at']) ?? DateTime.now(),
      );
}

class FeedEvent {
  FeedEvent({
    required this.id,
    required this.url,
    required this.title,
    required this.summary,
    required this.lang,
    required this.fetchedAt,
    required this.sourceId,
    required this.sourceName,
    required this.relevance,
    required this.matchedTopics,
    this.publishedAt,
  });

  final int id;
  final String url;
  final String title;
  final String summary;
  final String lang;
  final DateTime fetchedAt;
  final DateTime? publishedAt;
  final int sourceId;
  final String sourceName;
  final double relevance;
  final List<int> matchedTopics;

  factory FeedEvent.fromJson(Map<String, dynamic> j) => FeedEvent(
        id: (j['id'] as num).toInt(),
        url: (j['url'] as String?) ?? '',
        title: (j['title'] as String?) ?? '',
        summary: (j['summary'] as String?) ?? '',
        lang: (j['lang'] as String?) ?? 'zh',
        fetchedAt: _parseTime(j['fetched_at']) ?? DateTime.now(),
        publishedAt: _parseTime(j['published_at']),
        sourceId: (j['source_id'] as num?)?.toInt() ?? 0,
        sourceName: (j['source_name'] as String?) ?? '',
        relevance: (j['relevance'] as num?)?.toDouble() ?? 0,
        matchedTopics: ((j['matched_topics'] as List?) ?? const [])
            .map((e) => (e as num).toInt())
            .toList(),
      );
}

class GraphNode {
  GraphNode({
    required this.id,
    required this.type,
    required this.name,
    required this.weight,
    required this.lastSeenAt,
  });

  final int id;
  final String type;
  final String name;
  final double weight;
  final DateTime lastSeenAt;

  factory GraphNode.fromJson(Map<String, dynamic> j) => GraphNode(
        id: (j['id'] as num).toInt(),
        type: (j['type'] as String?) ?? 'concept',
        name: (j['name'] as String?) ?? '',
        weight: (j['weight'] as num?)?.toDouble() ?? 1.0,
        lastSeenAt: _parseTime(j['last_seen_at']) ?? DateTime.now(),
      );
}

class GraphEdge {
  GraphEdge({
    required this.id,
    required this.srcId,
    required this.dstId,
    required this.label,
    required this.weight,
  });

  final int id;
  final int srcId;
  final int dstId;
  final String label;
  final double weight;

  factory GraphEdge.fromJson(Map<String, dynamic> j) => GraphEdge(
        id: (j['id'] as num).toInt(),
        srcId: (j['src_id'] as num).toInt(),
        dstId: (j['dst_id'] as num).toInt(),
        label: (j['label'] as String?) ?? 'related',
        weight: (j['weight'] as num?)?.toDouble() ?? 1.0,
      );
}

class FeedGraph {
  FeedGraph({required this.nodes, required this.edges});

  final List<GraphNode> nodes;
  final List<GraphEdge> edges;

  bool get isEmpty => nodes.isEmpty;

  factory FeedGraph.fromJson(Map<String, dynamic> j) => FeedGraph(
        nodes: ((j['nodes'] as List?) ?? const [])
            .map((e) => GraphNode.fromJson(e as Map<String, dynamic>))
            .toList(),
        edges: ((j['edges'] as List?) ?? const [])
            .map((e) => GraphEdge.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}

DateTime? _parseTime(dynamic v) {
  if (v == null || v == '') return null;
  if (v is String) return DateTime.tryParse(v);
  return null;
}

/// 新闻源（给「源库管理」用）。
class NewsSource {
  NewsSource({
    required this.id,
    required this.name,
    required this.url,
    required this.category,
    required this.description,
    required this.enabled,
    required this.region,
    required this.lang,
  });

  final int id;
  final String name;
  final String url;
  final String category;
  final String description;
  final bool enabled;
  final String region;
  final String lang;

  factory NewsSource.fromJson(Map<String, dynamic> j) => NewsSource(
        id: (j['id'] as num).toInt(),
        name: (j['name'] as String?) ?? '',
        url: (j['url'] as String?) ?? '',
        category: (j['category'] as String?) ?? 'general',
        description: (j['description'] as String?) ?? '',
        enabled: j['enabled'] == true,
        region: (j['region'] as String?) ?? 'cn',
        lang: (j['lang'] as String?) ?? 'zh',
      );
}

/// AI 情报简报元数据（列表项用，正文走 HTML 接口）
class Briefing {
  Briefing({
    required this.id,
    required this.window,
    required this.title,
    required this.summary,
    required this.model,
    required this.eventCount,
    required this.clusterCount,
    required this.generatedAt,
  });

  final int id;
  final String window;
  final String title;
  final String summary;
  final String model;
  final int eventCount;
  final int clusterCount;
  final DateTime generatedAt;

  factory Briefing.fromJson(Map<String, dynamic> j) => Briefing(
        id: (j['id'] as num).toInt(),
        window: (j['window'] as String?) ?? '1h',
        title: (j['title'] as String?) ?? '',
        summary: (j['summary'] as String?) ?? '',
        model: (j['model'] as String?) ?? '',
        eventCount: (j['event_count'] as num?)?.toInt() ?? 0,
        clusterCount: (j['cluster_count'] as num?)?.toInt() ?? 0,
        generatedAt: _parseTime(j['generated_at']) ?? DateTime.now(),
      );
}

/// LLM 智能选源结果。
class RecommendResult {
  RecommendResult({required this.newlyEnabled, required this.reason});

  final List<NewsSource> newlyEnabled;
  final String reason;

  factory RecommendResult.fromJson(Map<String, dynamic> j) => RecommendResult(
        newlyEnabled: ((j['newly_enabled'] as List?) ?? const [])
            .map((e) => NewsSource.fromJson(Map<String, dynamic>.from(e)))
            .toList(),
        reason: (j['reason'] as String?) ?? '',
      );
}

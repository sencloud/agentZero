import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../models/feed.dart';

/// 周期性轮询事件流状态：心跳点 + 总数等聚合用。
final feedStatusProvider = StreamProvider.autoDispose<FeedStatus>((ref) async* {
  final dio = ref.watch(apiClientProvider).dio;
  while (true) {
    try {
      final res = await dio.get('/feed/status');
      final st = FeedStatus.fromJson(Map<String, dynamic>.from(res.data));
      yield st;
    } catch (_) {
      yield FeedStatus(
        running: false,
        sourcesTotal: 0,
        sourcesActive: 0,
        events24h: 0,
        entitiesTotal: 0,
        relationsTotal: 0,
        lastError: 'unreachable',
      );
    }
    await Future.delayed(const Duration(seconds: 12));
  }
});

final topicsProvider = FutureProvider.autoDispose<List<Topic>>((ref) async {
  final dio = ref.watch(apiClientProvider).dio;
  final res = await dio.get('/feed/topics');
  final list = (res.data['topics'] as List?) ?? const [];
  return list.map((e) => Topic.fromJson(Map<String, dynamic>.from(e))).toList();
});

final feedEventsProvider = FutureProvider.autoDispose<List<FeedEvent>>((ref) async {
  final dio = ref.watch(apiClientProvider).dio;
  final res = await dio.get('/feed/events', queryParameters: {'limit': 60});
  final list = (res.data['events'] as List?) ?? const [];
  return list.map((e) => FeedEvent.fromJson(Map<String, dynamic>.from(e))).toList();
});

final feedGraphProvider = FutureProvider.autoDispose<FeedGraph>((ref) async {
  final dio = ref.watch(apiClientProvider).dio;
  final res = await dio.get('/feed/graph', queryParameters: {'limit': 80});
  return FeedGraph.fromJson(Map<String, dynamic>.from(res.data));
});

final feedSourcesProvider = FutureProvider.autoDispose<List<NewsSource>>((ref) async {
  final dio = ref.watch(apiClientProvider).dio;
  final res = await dio.get('/feed/sources');
  final list = (res.data['sources'] as List?) ?? const [];
  return list.map((e) => NewsSource.fromJson(Map<String, dynamic>.from(e))).toList();
});

class FeedActions {
  FeedActions(this._ref);
  final Ref _ref;

  Future<Topic> addTopic(String name, {double weight = 1.0}) async {
    final dio = _ref.read(apiClientProvider).dio;
    final res = await dio.post('/feed/topics', data: {
      'name': name,
      'weight': weight,
    });
    _ref.invalidate(topicsProvider);
    _ref.invalidate(feedEventsProvider);
    return Topic.fromJson(Map<String, dynamic>.from(res.data));
  }

  Future<void> deleteTopic(int id) async {
    final dio = _ref.read(apiClientProvider).dio;
    await dio.delete('/feed/topics/$id');
    _ref.invalidate(topicsProvider);
    _ref.invalidate(feedEventsProvider);
    _ref.invalidate(feedGraphProvider);
  }

  Future<void> refresh() async {
    final dio = _ref.read(apiClientProvider).dio;
    await dio.post('/feed/refresh');
    _ref.invalidate(feedStatusProvider);
    _ref.invalidate(feedEventsProvider);
    _ref.invalidate(feedGraphProvider);
  }

  Future<RecommendResult> recommendSources() async {
    final dio = _ref.read(apiClientProvider).dio;
    final res = await dio.post('/feed/sources/recommend');
    _ref.invalidate(feedSourcesProvider);
    _ref.invalidate(feedStatusProvider);
    return RecommendResult.fromJson(Map<String, dynamic>.from(res.data));
  }

  Future<void> toggleSources(List<int> ids, bool enabled) async {
    final dio = _ref.read(apiClientProvider).dio;
    await dio.post('/feed/sources/toggle', data: {
      'ids': ids,
      'enabled': enabled,
    });
    _ref.invalidate(feedSourcesProvider);
    _ref.invalidate(feedStatusProvider);
  }
}

final feedActionsProvider = Provider<FeedActions>((ref) => FeedActions(ref));

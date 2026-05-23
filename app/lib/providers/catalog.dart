import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../models/agent.dart';
import '../models/category.dart';
import '../models/review.dart';
import '../models/today_card.dart';

class TodayFeed {
  TodayFeed({required this.cards, required this.featured});
  final List<TodayCard> cards;
  final List<Agent> featured;
}

final todayFeedProvider = FutureProvider.autoDispose<TodayFeed>((ref) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get('/feed/today');
  final data = r.data as Map<String, dynamic>;
  return TodayFeed(
    cards: (data['cards'] as List).map((e) => TodayCard.fromJson(e as Map<String, dynamic>)).toList(),
    featured: (data['featured'] as List).map((e) => Agent.fromJson(e as Map<String, dynamic>)).toList(),
  );
});

final categoriesProvider = FutureProvider.autoDispose<List<AgentCategory>>((ref) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get('/categories');
  final items = (r.data['items'] as List);
  return items.map((e) => AgentCategory.fromJson(e as Map<String, dynamic>)).toList();
});

class AgentListQuery {
  AgentListQuery({this.category, this.query, this.sort, this.featured});
  final String? category;
  final String? query;
  final String? sort;
  final bool? featured;

  Map<String, dynamic> toParams() => {
        if (category != null && category!.isNotEmpty) 'category': category,
        if (query != null && query!.isNotEmpty) 'q': query,
        if (sort != null && sort!.isNotEmpty) 'sort': sort,
        if (featured == true) 'featured': '1',
      };

  @override
  bool operator ==(Object other) =>
      other is AgentListQuery &&
      other.category == category &&
      other.query == query &&
      other.sort == sort &&
      other.featured == featured;

  @override
  int get hashCode => Object.hash(category, query, sort, featured);
}

final agentListProvider = FutureProvider.autoDispose.family<List<Agent>, AgentListQuery>((ref, q) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get('/agents', queryParameters: q.toParams());
  final items = (r.data['items'] as List);
  return items.map((e) => Agent.fromJson(e as Map<String, dynamic>)).toList();
});

final agentDetailProvider = FutureProvider.autoDispose.family<Agent, String>((ref, slug) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get('/agents/$slug');
  return Agent.fromJson(r.data as Map<String, dynamic>);
});

final installedAgentsProvider = FutureProvider.autoDispose<List<Agent>>((ref) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get('/me/installed');
  final items = (r.data['items'] as List);
  return items.map((e) => Agent.fromJson(e as Map<String, dynamic>)).toList();
});

final agentReviewsProvider = FutureProvider.autoDispose.family<List<Review>, String>((ref, slug) async {
  final api = ref.watch(apiClientProvider);
  final r = await api.dio.get('/agents/$slug/reviews');
  final items = (r.data['items'] as List);
  return items.map((e) => Review.fromJson(e as Map<String, dynamic>)).toList();
});

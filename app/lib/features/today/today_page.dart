import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../models/agent.dart';
import '../../models/today_card.dart';
import '../../providers/catalog.dart';
import '../../widgets/agent_icon.dart';
import '../../widgets/feature_card.dart';

class TodayPage extends ConsumerWidget {
  const TodayPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final feed = ref.watch(todayFeedProvider);
    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: feed.when(
          loading: () => const Center(child: CupertinoActivityIndicator(radius: 14)),
          error: (e, _) => _ErrorView(
            message: '加载失败：$e',
            onRetry: () => ref.invalidate(todayFeedProvider),
          ),
          data: (data) => _TodayContent(cards: data.cards, featured: data.featured),
        ),
      ),
    );
  }
}

class _TodayContent extends StatelessWidget {
  const _TodayContent({required this.cards, required this.featured});
  final List<TodayCard> cards;
  final List<Agent> featured;

  @override
  Widget build(BuildContext context) {
    final dateLabel = DateFormat('MMMd日 EEEE', 'zh_CN').format(DateTime.now());
    return CustomScrollView(
      slivers: [
        SliverPadding(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 0),
          sliver: SliverToBoxAdapter(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  dateLabel.toUpperCase(),
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    color: Theme.of(context).hintColor,
                    letterSpacing: 1.2,
                  ),
                ),
                const SizedBox(height: 4),
                Text('今日', style: Theme.of(context).textTheme.displayLarge),
                const SizedBox(height: 8),
              ],
            ),
          ),
        ),
        SliverList(
          delegate: SliverChildBuilderDelegate(
            (context, i) => FeatureCard(card: cards[i]),
            childCount: cards.length,
          ),
        ),
        if (featured.isNotEmpty)
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(20, 24, 20, 8),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('本周精选', style: Theme.of(context).textTheme.displaySmall),
                  GestureDetector(
                    onTap: () => GoRouter.of(context).go('/apps'),
                    child: const Text('全部',
                        style: TextStyle(color: Color(0xFF0A84FF), fontSize: 15, fontWeight: FontWeight.w600)),
                  ),
                ],
              ),
            ),
          ),
        if (featured.isNotEmpty)
          SliverToBoxAdapter(
            child: SizedBox(
              height: 230,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 20),
                itemBuilder: (_, i) => _FeaturedAppTile(agent: featured[i]),
                separatorBuilder: (_, _) => const SizedBox(width: 16),
                itemCount: featured.length,
              ),
            ),
          ),
        const SliverToBoxAdapter(child: SizedBox(height: 40)),
      ],
    );
  }
}

class _FeaturedAppTile extends StatelessWidget {
  const _FeaturedAppTile({required this.agent});
  final Agent agent;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => context.push('/agent/${agent.slug}'),
      child: SizedBox(
        width: 168,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AgentIcon(iconUrl: agent.iconUrl, size: 100),
            const SizedBox(height: 12),
            Text(agent.name,
                style: Theme.of(context).textTheme.titleMedium,
                maxLines: 1,
                overflow: TextOverflow.ellipsis),
            const SizedBox(height: 2),
            Text(agent.tagline,
                style: Theme.of(context).textTheme.bodyMedium,
                maxLines: 2,
                overflow: TextOverflow.ellipsis),
            const SizedBox(height: 8),
            if (agent.featureBadge.isNotEmpty)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: BoxDecoration(
                  color: const Color(0xFF0A84FF).withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Text(
                  agent.featureBadge,
                  style: const TextStyle(
                    color: Color(0xFF0A84FF),
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});
  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(CupertinoIcons.exclamationmark_circle, size: 38, color: Color(0xFF8E8E93)),
          const SizedBox(height: 12),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 32),
            child: Text(message, style: Theme.of(context).textTheme.bodyMedium, textAlign: TextAlign.center),
          ),
          const SizedBox(height: 12),
          CupertinoButton(onPressed: onRetry, child: const Text('重试')),
        ],
      ),
    );
  }
}

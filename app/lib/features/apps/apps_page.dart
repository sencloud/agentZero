import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/icon_mapper.dart';
import '../../models/category.dart';
import '../../providers/catalog.dart';
import '../../widgets/agent_row.dart';

class AppsPage extends ConsumerWidget {
  const AppsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cats = ref.watch(categoriesProvider);
    final featured = ref.watch(agentListProvider(AgentListQuery(featured: true)));
    final top = ref.watch(agentListProvider(AgentListQuery(sort: 'top')));

    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: CustomScrollView(
          slivers: [
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 8, 20, 16),
              sliver: SliverToBoxAdapter(
                child: Text('应用', style: Theme.of(context).textTheme.displayLarge),
              ),
            ),
            SliverToBoxAdapter(
              child: _SectionHeader(title: '编辑推荐', onMore: () {}),
            ),
            featured.when(
              loading: () => const SliverToBoxAdapter(child: _LoaderBar()),
              error: (e, _) => SliverToBoxAdapter(child: _ErrorBar(message: '$e')),
              data: (items) => SliverToBoxAdapter(
                child: SizedBox(
                  height: 240,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    padding: const EdgeInsets.symmetric(horizontal: 20),
                    itemCount: items.length,
                    separatorBuilder: (_, _) => const SizedBox(width: 16),
                    itemBuilder: (_, i) {
                      final a = items[i];
                      return GestureDetector(
                        onTap: () => context.push('/agent/${a.slug}'),
                        child: SizedBox(
                          width: 280,
                          child: Container(
                            padding: const EdgeInsets.all(18),
                            decoration: BoxDecoration(
                              color: Theme.of(context).cardColor,
                              borderRadius: BorderRadius.circular(20),
                              boxShadow: [
                                BoxShadow(
                                  color: Colors.black.withValues(alpha: 0.06),
                                  blurRadius: 18,
                                  offset: const Offset(0, 6),
                                ),
                              ],
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text((a.featureBadge.isNotEmpty ? a.featureBadge : '编辑精选').toUpperCase(),
                                    style: const TextStyle(
                                      fontSize: 12,
                                      fontWeight: FontWeight.w700,
                                      letterSpacing: 1.2,
                                      color: Color(0xFF0A84FF),
                                    )),
                                const SizedBox(height: 6),
                                Text(a.name,
                                    style: Theme.of(context).textTheme.headlineMedium,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis),
                                const SizedBox(height: 4),
                                Text(a.tagline,
                                    style: Theme.of(context).textTheme.bodyMedium,
                                    maxLines: 2,
                                    overflow: TextOverflow.ellipsis),
                                const Spacer(),
                                Row(
                                  children: [
                                    Expanded(child: AgentRow(agent: a)),
                                  ],
                                ),
                              ],
                            ),
                          ),
                        ),
                      );
                    },
                  ),
                ),
              ),
            ),
            const SliverToBoxAdapter(child: SizedBox(height: 16)),
            SliverToBoxAdapter(
              child: _SectionHeader(title: '分类浏览', onMore: () {}),
            ),
            cats.when(
              loading: () => const SliverToBoxAdapter(child: _LoaderBar()),
              error: (e, _) => SliverToBoxAdapter(child: _ErrorBar(message: '$e')),
              data: (items) => SliverToBoxAdapter(child: _CategoryGrid(items: items)),
            ),
            const SliverToBoxAdapter(child: SizedBox(height: 16)),
            SliverToBoxAdapter(child: _SectionHeader(title: '热门榜单', onMore: () {})),
            top.when(
              loading: () => const SliverToBoxAdapter(child: _LoaderBar()),
              error: (e, _) => SliverToBoxAdapter(child: _ErrorBar(message: '$e')),
              data: (items) => SliverPadding(
                padding: const EdgeInsets.symmetric(horizontal: 20),
                sliver: SliverList.separated(
                  itemBuilder: (_, i) => AgentRow(agent: items[i], rank: i + 1),
                  separatorBuilder: (_, _) => const Divider(),
                  itemCount: items.length.clamp(0, 10),
                ),
              ),
            ),
            const SliverToBoxAdapter(child: SizedBox(height: 40)),
          ],
        ),
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.title, required this.onMore});
  final String title;
  final VoidCallback onMore;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 18, 20, 10),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(title, style: Theme.of(context).textTheme.displaySmall),
          GestureDetector(
            onTap: onMore,
            child: const Text('查看全部',
                style: TextStyle(color: Color(0xFF0A84FF), fontSize: 15, fontWeight: FontWeight.w600)),
          ),
        ],
      ),
    );
  }
}

class _CategoryGrid extends StatelessWidget {
  const _CategoryGrid({required this.items});
  final List<AgentCategory> items;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Column(
        children: [
          for (var i = 0; i < items.length; i += 2)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 6),
              child: Row(
                children: [
                  Expanded(child: _CategoryTile(item: items[i])),
                  const SizedBox(width: 12),
                  Expanded(
                    child: i + 1 < items.length
                        ? _CategoryTile(item: items[i + 1])
                        : const SizedBox.shrink(),
                  ),
                ],
              ),
            )
        ],
      ),
    );
  }
}

class _CategoryTile extends StatelessWidget {
  const _CategoryTile({required this.item});
  final AgentCategory item;

  @override
  Widget build(BuildContext context) {
    final color = CategoryIconSpec.colorFor(item.color);
    return GestureDetector(
      onTap: () => context.push('/category/${item.slug}?name=${Uri.encodeComponent(item.name)}'),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 18),
        decoration: BoxDecoration(
          color: Theme.of(context).cardColor,
          borderRadius: BorderRadius.circular(16),
        ),
        child: Row(
          children: [
            Container(
              width: 38,
              height: 38,
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(10),
              ),
              alignment: Alignment.center,
              child: Icon(CategoryIconSpec.iconFor(item.icon), color: color, size: 22),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                item.name,
                style: Theme.of(context).textTheme.titleMedium,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const Icon(CupertinoIcons.chevron_right, size: 14, color: Color(0xFF8E8E93)),
          ],
        ),
      ),
    );
  }
}

class _LoaderBar extends StatelessWidget {
  const _LoaderBar();
  @override
  Widget build(BuildContext context) => const Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: Center(child: CupertinoActivityIndicator()),
      );
}

class _ErrorBar extends StatelessWidget {
  const _ErrorBar({required this.message});
  final String message;
  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.all(20),
        child: Text('加载失败：$message', style: Theme.of(context).textTheme.bodyMedium),
      );
}

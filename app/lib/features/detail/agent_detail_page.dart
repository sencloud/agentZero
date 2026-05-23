import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../models/agent.dart';
import '../../providers/auth.dart';
import '../../providers/catalog.dart';
import '../../providers/install.dart';
import '../../widgets/agent_icon.dart';

class AgentDetailPage extends ConsumerWidget {
  const AgentDetailPage({super.key, required this.slug});
  final String slug;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final detail = ref.watch(agentDetailProvider(slug));
    final reviews = ref.watch(agentReviewsProvider(slug));

    return Scaffold(
      appBar: AppBar(title: const Text(''), elevation: 0),
      body: detail.when(
        loading: () => const Center(child: CupertinoActivityIndicator()),
        error: (e, _) => Center(child: Text('加载失败：$e')),
        data: (a) => ListView(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 40),
          children: [
            _Header(agent: a),
            const SizedBox(height: 24),
            _StatsRow(agent: a),
            const SizedBox(height: 28),
            Text('截图', style: Theme.of(context).textTheme.headlineMedium),
            const SizedBox(height: 12),
            SizedBox(
              height: 420,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: a.screenshots.length,
                separatorBuilder: (_, _) => const SizedBox(width: 12),
                itemBuilder: (_, i) => ClipRRect(
                  borderRadius: BorderRadius.circular(20),
                  child: CachedNetworkImage(
                    imageUrl: a.screenshots[i],
                    width: 220,
                    fit: BoxFit.cover,
                    placeholder: (_, _) => Container(width: 220, color: const Color(0xFFE5E5EA)),
                  ),
                ),
              ),
            ),
            const SizedBox(height: 28),
            Text('简介', style: Theme.of(context).textTheme.headlineMedium),
            const SizedBox(height: 8),
            Text(a.description, style: Theme.of(context).textTheme.bodyLarge),
            const SizedBox(height: 28),
            if (a.capabilities.isNotEmpty) ...[
              Text('它能做什么', style: Theme.of(context).textTheme.headlineMedium),
              const SizedBox(height: 12),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: a.capabilities
                    .map((c) => Container(
                          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                          decoration: BoxDecoration(
                            color: Theme.of(context).cardColor,
                            borderRadius: BorderRadius.circular(20),
                          ),
                          child: Text(c, style: Theme.of(context).textTheme.bodyMedium),
                        ))
                    .toList(),
              ),
              const SizedBox(height: 28),
            ],
            if (a.updatedNotes.isNotEmpty) ...[
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('最近更新', style: Theme.of(context).textTheme.headlineMedium),
                  Text('版本 ${a.version}', style: Theme.of(context).textTheme.bodySmall),
                ],
              ),
              const SizedBox(height: 8),
              Text(a.updatedNotes, style: Theme.of(context).textTheme.bodyLarge),
              const SizedBox(height: 28),
            ],
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('用户评价', style: Theme.of(context).textTheme.headlineMedium),
                Text(
                  '${a.rating.toStringAsFixed(1)} · ${a.ratingCount} 个评分',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
              ],
            ),
            const SizedBox(height: 12),
            reviews.when(
              loading: () => const Padding(
                padding: EdgeInsets.symmetric(vertical: 16),
                child: Center(child: CupertinoActivityIndicator()),
              ),
              error: (e, _) => Text('$e'),
              data: (items) {
                if (items.isEmpty) {
                  return Container(
                    padding: const EdgeInsets.symmetric(vertical: 20),
                    alignment: Alignment.center,
                    child: Text(
                      '还没有用户评价，做第一个评价的人吧',
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                  );
                }
                return Column(
                  children: items
                      .map((r) => Container(
                            margin: const EdgeInsets.symmetric(vertical: 6),
                            padding: const EdgeInsets.all(14),
                            decoration: BoxDecoration(
                              color: Theme.of(context).cardColor,
                              borderRadius: BorderRadius.circular(14),
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    Text(r.nickname, style: Theme.of(context).textTheme.titleMedium),
                                    const SizedBox(width: 8),
                                    _Stars(rating: r.rating.toDouble()),
                                    const Spacer(),
                                    Text(DateFormat('y/M/d').format(r.createdAt),
                                        style: Theme.of(context).textTheme.bodySmall),
                                  ],
                                ),
                                const SizedBox(height: 8),
                                Text(r.title, style: Theme.of(context).textTheme.titleSmall),
                                const SizedBox(height: 4),
                                Text(r.body, style: Theme.of(context).textTheme.bodyLarge),
                              ],
                            ),
                          ))
                      .toList(),
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _Header extends ConsumerStatefulWidget {
  const _Header({required this.agent});
  final Agent agent;
  @override
  ConsumerState<_Header> createState() => _HeaderState();
}

class _HeaderState extends ConsumerState<_Header> {
  bool _busy = false;

  Future<void> _onInstall() async {
    final auth = ref.read(authProvider);
    if (!auth.isSignedIn) {
      context.push('/sign-in');
      return;
    }
    setState(() => _busy = true);
    try {
      final ctrl = ref.read(installControllerProvider);
      if (widget.agent.installed) {
        await ctrl.uninstall(widget.agent.slug);
      } else {
        await ctrl.install(widget.agent.slug);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final a = widget.agent;
    final installed = a.installed;
    final label = installed ? '打开' : (a.isFree ? '获取' : '￥${(a.priceCents / 100).toStringAsFixed(2)}');
    final primaryAction = installed ? () => context.push('/agent/${a.slug}/chat') : _onInstall;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AgentIcon(iconUrl: a.iconUrl, size: 110),
        const SizedBox(width: 16),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(a.name, style: Theme.of(context).textTheme.displaySmall),
              const SizedBox(height: 4),
              Text(a.tagline, style: Theme.of(context).textTheme.bodyMedium),
              const SizedBox(height: 12),
              Row(
                children: [
                  SizedBox(
                    height: 32,
                    child: CupertinoButton(
                      padding: const EdgeInsets.symmetric(horizontal: 22),
                      borderRadius: BorderRadius.circular(20),
                      color: const Color(0xFF0A84FF),
                      onPressed: _busy ? null : primaryAction,
                      child: _busy
                          ? const CupertinoActivityIndicator(color: Colors.white, radius: 8)
                          : Text(label,
                              style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700)),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Container(
                    width: 36,
                    height: 32,
                    decoration: BoxDecoration(
                      color: const Color(0xFFE5E5EA),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: const Icon(CupertinoIcons.share, size: 18, color: Color(0xFF0A84FF)),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _StatsRow extends StatelessWidget {
  const _StatsRow({required this.agent});
  final Agent agent;

  String _installs(int n) {
    if (n >= 10000) return '${(n / 10000).toStringAsFixed(1)}万';
    return '$n';
  }

  String _size(int b) {
    final mb = b / (1024 * 1024);
    return '${mb.toStringAsFixed(1)} MB';
  }

  @override
  Widget build(BuildContext context) {
    final divider = Container(width: 0.5, height: 36, color: const Color(0xFFC7C7CC));
    Widget cell(String top, String bottom, {Widget? topWidget}) {
      return Expanded(
        child: Column(
          children: [
            topWidget ??
                Text(top,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700)),
            const SizedBox(height: 4),
            Text(bottom, style: Theme.of(context).textTheme.bodySmall),
          ],
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.symmetric(vertical: 14),
      decoration: BoxDecoration(
        color: Theme.of(context).cardColor,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          cell('${agent.rating.toStringAsFixed(1)}★',
              '${agent.ratingCount} 评分',
              topWidget: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(agent.rating.toStringAsFixed(1),
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700)),
                  const SizedBox(width: 2),
                  const Icon(CupertinoIcons.star_fill, size: 13, color: Color(0xFFFFCC00)),
                ],
              )),
          divider,
          cell('#${agent.id}', agent.categoryName),
          divider,
          cell(_installs(agent.installCount), '次安装'),
          divider,
          cell(_size(agent.sizeBytes), '体积'),
        ],
      ),
    );
  }
}

class _Stars extends StatelessWidget {
  const _Stars({required this.rating});
  final double rating;
  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(5, (i) {
        final filled = i < rating;
        return Icon(
          filled ? CupertinoIcons.star_fill : CupertinoIcons.star,
          size: 12,
          color: const Color(0xFFFFCC00),
        );
      }),
    );
  }
}

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../core/theme.dart';
import '../../models/feed.dart';
import '../../providers/feed.dart';
import 'feed_graph_canvas.dart';

/// 事件流页：话题 chips + 实体图谱画布 + 命中事件 timeline。
///
/// 顶部状态条提示用户当前的 worker 状态、24h 入库事件数、节点/关系总数。
class FeedPage extends ConsumerWidget {
  const FeedPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(feedStatusProvider).value;
    final topics = ref.watch(topicsProvider);
    final events = ref.watch(feedEventsProvider);
    final graph = ref.watch(feedGraphProvider);
    final actions = ref.read(feedActionsProvider);

    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(CupertinoIcons.back, color: AppTheme.paper),
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go('/');
            }
          },
        ),
        title: const Text('事件流',
            style: TextStyle(color: AppTheme.paper, fontSize: 14, letterSpacing: 4)),
        actions: [
          IconButton(
            tooltip: '立即刷新',
            icon: const Icon(CupertinoIcons.refresh, color: AppTheme.paper, size: 18),
            onPressed: () => actions.refresh(),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _StatusBar(status: status),
            _TopicsBar(
              topicsAsync: topics,
              onAdd: () => _showAddTopic(context, actions),
              onDelete: (id) async {
                await actions.deleteTopic(id);
              },
            ),
            Expanded(
              child: graph.when(
                loading: () => const Center(
                  child: SizedBox(
                    width: 22,
                    height: 22,
                    child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
                  ),
                ),
                error: (e, _) => Center(
                  child: Text('图谱加载失败：$e',
                      style: const TextStyle(color: AppTheme.redline, fontSize: 12)),
                ),
                data: (g) => FeedGraphCanvas(graph: g),
              ),
            ),
            const Divider(color: AppTheme.graphite, height: 1),
            SizedBox(
              height: 220,
              child: events.when(
                loading: () => const Center(
                  child: SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
                  ),
                ),
                error: (e, _) => Center(
                  child: Text('事件列表加载失败：$e',
                      style: const TextStyle(color: AppTheme.redline, fontSize: 12)),
                ),
                data: (list) => _EventList(events: list),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _showAddTopic(BuildContext context, FeedActions actions) async {
    final ctrl = TextEditingController();
    final added = await showModalBottomSheet<String>(
      context: context,
      backgroundColor: AppTheme.ink,
      isScrollControlled: true,
      builder: (ctx) {
        final viewInsets = MediaQuery.of(ctx).viewInsets;
        return Padding(
          padding: EdgeInsets.fromLTRB(20, 20, 20, 20 + viewInsets.bottom),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Text('添加关注话题',
                  style: TextStyle(
                    color: AppTheme.paper,
                    fontSize: 14,
                    letterSpacing: 4,
                    fontWeight: FontWeight.w700,
                  )),
              const SizedBox(height: 6),
              const Text(
                '可以是公司、人物、产品、行业概念等。命中标题/摘要的事件会自动进入你的事件流。',
                style: TextStyle(color: AppTheme.muted, fontSize: 11, height: 1.6),
              ),
              const SizedBox(height: 14),
              TextField(
                controller: ctrl,
                autofocus: true,
                style: const TextStyle(color: AppTheme.paper, fontSize: 14, letterSpacing: 2),
                decoration: const InputDecoration(
                  hintText: '如：OpenAI / 半导体 / 黄仁勋',
                  hintStyle: TextStyle(color: AppTheme.muted, fontSize: 13),
                  enabledBorder: UnderlineInputBorder(
                    borderSide: BorderSide(color: AppTheme.graphite),
                  ),
                  focusedBorder: UnderlineInputBorder(
                    borderSide: BorderSide(color: AppTheme.paper),
                  ),
                ),
                onSubmitted: (v) => Navigator.of(ctx).pop(v.trim()),
              ),
              const SizedBox(height: 18),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: () => Navigator.of(ctx).pop(),
                      child: const Text('取消'),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: ElevatedButton(
                      onPressed: () => Navigator.of(ctx).pop(ctrl.text.trim()),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppTheme.redline,
                        foregroundColor: AppTheme.paper,
                      ),
                      child: const Text('加入'),
                    ),
                  ),
                ],
              ),
            ],
          ),
        );
      },
    );
    if (added != null && added.isNotEmpty) {
      await actions.addTopic(added);
    }
  }
}

class _StatusBar extends StatelessWidget {
  const _StatusBar({required this.status});
  final FeedStatus? status;

  @override
  Widget build(BuildContext context) {
    final st = status;
    final state = st?.running == true
        ? '在线'
        : (st?.lastError.isNotEmpty == true ? '异常' : '空闲');
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: AppTheme.graphite, width: 0.6)),
      ),
      child: Wrap(
        spacing: 12,
        runSpacing: 8,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          _Stat(label: '状态', value: state),
          _Stat(label: '24H 事件', value: '${st?.events24h ?? 0}'),
          _Stat(label: '节点', value: '${st?.entitiesTotal ?? 0}'),
          _Stat(label: '关系', value: '${st?.relationsTotal ?? 0}'),
          _Stat(label: '源', value: '${st?.sourcesActive ?? 0}/${st?.sourcesTotal ?? 0}'),
        ],
      ),
    );
  }
}

class _Stat extends StatelessWidget {
  const _Stat({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(label,
            style: const TextStyle(
              color: AppTheme.muted,
              fontSize: 10,
              letterSpacing: 2,
              fontFamilyFallback: AppTheme.monoFallback,
            )),
        const SizedBox(width: 4),
        Text(value,
            style: const TextStyle(
              color: AppTheme.paper,
              fontSize: 12,
              letterSpacing: 1.5,
              fontFamilyFallback: AppTheme.monoFallback,
            )),
      ],
    );
  }
}

class _TopicsBar extends StatelessWidget {
  const _TopicsBar({required this.topicsAsync, required this.onAdd, required this.onDelete});
  final AsyncValue<List<Topic>> topicsAsync;
  final VoidCallback onAdd;
  final Future<void> Function(int id) onDelete;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: AppTheme.graphite, width: 0.6)),
      ),
      child: topicsAsync.when(
        loading: () => const SizedBox(height: 28),
        error: (e, _) => Text('话题加载失败：$e',
            style: const TextStyle(color: AppTheme.redline, fontSize: 11)),
        data: (topics) {
          return Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final t in topics) _TopicChip(topic: t, onDelete: () => onDelete(t.id)),
              _AddTopicChip(onTap: onAdd),
            ],
          );
        },
      ),
    );
  }
}

class _TopicChip extends StatelessWidget {
  const _TopicChip({required this.topic, required this.onDelete});
  final Topic topic;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onLongPress: onDelete,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: AppTheme.carbon,
          border: Border.all(color: AppTheme.paper.withValues(alpha: 0.4), width: 0.8),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(topic.name,
                style: const TextStyle(color: AppTheme.paper, fontSize: 12, letterSpacing: 2)),
            const SizedBox(width: 6),
            GestureDetector(
              onTap: onDelete,
              child: const Icon(CupertinoIcons.xmark, size: 10, color: AppTheme.muted),
            ),
          ],
        ),
      ),
    );
  }
}

class _AddTopicChip extends StatelessWidget {
  const _AddTopicChip({required this.onTap});
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          border: Border.all(color: AppTheme.redline, width: 0.8),
        ),
        child: const Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(CupertinoIcons.add, size: 12, color: AppTheme.redline),
            SizedBox(width: 4),
            Text('添加话题',
                style: TextStyle(
                  color: AppTheme.redline,
                  fontSize: 11,
                  letterSpacing: 2,
                  fontFamilyFallback: AppTheme.monoFallback,
                )),
          ],
        ),
      ),
    );
  }
}

class _EventList extends StatelessWidget {
  const _EventList({required this.events});
  final List<FeedEvent> events;

  @override
  Widget build(BuildContext context) {
    if (events.isEmpty) {
      return const Center(
        child: Text(
          '尚无命中事件\n添加关注话题，并等待下一轮抓取',
          textAlign: TextAlign.center,
          style: TextStyle(color: AppTheme.muted, fontSize: 12, height: 1.6, letterSpacing: 2),
        ),
      );
    }
    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      itemCount: events.length,
      separatorBuilder: (_, _) => const SizedBox(height: 10),
      itemBuilder: (_, i) => _EventTile(event: events[i]),
    );
  }
}

class _EventTile extends StatelessWidget {
  const _EventTile({required this.event});
  final FeedEvent event;

  @override
  Widget build(BuildContext context) {
    final t = event.publishedAt ?? event.fetchedAt;
    final tStr = '${t.month.toString().padLeft(2, '0')}/${t.day.toString().padLeft(2, '0')} '
        '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
    return InkWell(
      onTap: () async {
        final uri = Uri.tryParse(event.url);
        if (uri != null) {
          await launchUrl(uri, mode: LaunchMode.externalApplication);
        }
      },
      child: Container(
        padding: const EdgeInsets.all(10),
        decoration: BoxDecoration(
          color: AppTheme.carbon,
          border: Border.all(color: AppTheme.graphite, width: 0.6),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(event.sourceName,
                    style: const TextStyle(
                      color: AppTheme.amber,
                      fontSize: 10,
                      letterSpacing: 2,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
                const SizedBox(width: 8),
                Text(tStr,
                    style: const TextStyle(
                      color: AppTheme.muted,
                      fontSize: 10,
                      letterSpacing: 1.5,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
                const Spacer(),
                Text('relevance ${event.relevance.toStringAsFixed(1)}',
                    style: const TextStyle(
                      color: AppTheme.muted,
                      fontSize: 10,
                      letterSpacing: 1,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
              ],
            ),
            const SizedBox(height: 6),
            Text(event.title,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  color: AppTheme.paper,
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  height: 1.4,
                )),
            if (event.summary.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(event.summary,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(color: AppTheme.pen, fontSize: 11, height: 1.5)),
            ],
          ],
        ),
      ),
    );
  }
}

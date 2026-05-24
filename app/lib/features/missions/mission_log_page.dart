import 'dart:math' as math;

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../core/theme.dart';
import '../../models/mission.dart';
import '../../providers/auth.dart';
import '../../providers/missions.dart';
import '../feed/feed_status_chip.dart';

/// 任务簿主屏（M3b）。
///
/// 三种状态：
///   - 未登录 → 显示 SignInGate 引导
///   - 已登录但还没派遣过 → 显示空态 + 派遣 CTA
///   - 已登录且有任务 → 进行中 / 归档两段
class MissionLogPage extends ConsumerWidget {
  const MissionLogPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    // 启动期：先把本地 token 验过再决定显示 sign-in 还是任务簿，
    // 避免登录态打开 App 时 sign-in 页闪一下。
    if (!auth.bootstrapped) {
      return const _BootstrapSplash();
    }
    if (!auth.isSignedIn) {
      return const _SignInGate();
    }
    return _MissionLogShell(child: _MissionList());
  }
}

class _BootstrapSplash extends StatelessWidget {
  const _BootstrapSplash();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 84,
                height: 84,
                decoration: BoxDecoration(
                  border: Border.all(color: AppTheme.graphite, width: 0.8),
                ),
                child: Image.asset('assets/branding/app_icon.png', fit: BoxFit.cover),
              ),
              const SizedBox(height: 24),
              const Text(
                '代号零',
                style: TextStyle(
                  color: AppTheme.paper,
                  fontSize: 22,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 8,
                ),
              ),
              const SizedBox(height: 20),
              const SizedBox(
                width: 22,
                height: 22,
                child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SignInGate extends StatelessWidget {
  const _SignInGate();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                AppDecor.stamp('UNAUTHENTICATED', border: AppTheme.redline, color: AppTheme.redline),
                const SizedBox(height: 28),
                const Text(
                  '需要先完成接入',
                  style: TextStyle(
                    color: AppTheme.paper,
                    fontSize: 22,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 4,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  '行动局未登记你的身份。先去办入职手续，再回来翻任务簿。',
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppTheme.pen),
                ),
                const SizedBox(height: 28),
                FilledButton(
                  onPressed: () => context.push('/sign-in'),
                  child: const Text('前往登记 →'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _MissionLogShell extends StatelessWidget {
  const _MissionLogShell({required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        titleSpacing: 12,
        title: Row(
          children: [
            Container(
              width: 28,
              height: 28,
              decoration: BoxDecoration(border: Border.all(color: AppTheme.graphite, width: 0.8)),
              child: Image.asset('assets/branding/app_icon.png', fit: BoxFit.cover),
            ),
            const SizedBox(width: 10),
            const Text('任务簿', style: TextStyle(color: AppTheme.paper, fontSize: 14, letterSpacing: 4)),
            const Spacer(),
            const FeedStatusChip(),
          ],
        ),
      ),
      body: SafeArea(top: false, child: child),
      floatingActionButtonLocation: FloatingActionButtonLocation.endFloat,
      floatingActionButton: _DispatchFab(),
    );
  }
}

class _DispatchFab extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppTheme.redline,
        border: Border.all(color: AppTheme.paper, width: 1),
      ),
      child: InkWell(
        onTap: () => context.push('/missions/new'),
        child: const Padding(
          padding: EdgeInsets.symmetric(horizontal: 22, vertical: 14),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(CupertinoIcons.paperplane_fill, color: AppTheme.paper, size: 16),
              SizedBox(width: 10),
              Text(
                '派遣新行动',
                style: TextStyle(
                  color: AppTheme.paper,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 4,
                  fontSize: 12,
                  fontFamilyFallback: AppTheme.monoFallback,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MissionList extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final missions = ref.watch(missionsListProvider);
    return missions.when(
      loading: () => const _Loading(),
      error: (e, _) => _ErrorView(message: '$e', onRetry: () => ref.invalidate(missionsListProvider)),
      data: (items) {
        if (items.isEmpty) return const _EmptyState();
        // 按 series_id 折叠：每个卷宗只露出最新一卷做"代表"。
        final groups = <String, List<Mission>>{};
        for (final m in items) {
          groups.putIfAbsent(m.seriesId, () => []).add(m);
        }
        for (final g in groups.values) {
          g.sort((a, b) => a.seriesSeq.compareTo(b.seriesSeq));
        }
        final reps = groups.values.map((g) => g.last).toList()
          ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
        final running = reps.where((m) => !m.status.isTerminal).toList();
        final archived = reps.where((m) => m.status.isTerminal).toList();

        return RefreshIndicator(
          color: AppTheme.paper,
          backgroundColor: AppTheme.carbon,
          onRefresh: () async => ref.invalidate(missionsListProvider),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 120),
            children: [
              if (running.isNotEmpty) ...[
                AppDecor.sectionRule('进行中  ·  ${running.length}'),
                const SizedBox(height: 4),
                for (final m in running)
                  _SwipeableMissionCard(mission: m, seriesSize: groups[m.seriesId]!.length),
                const SizedBox(height: 24),
              ],
              if (archived.isNotEmpty) ...[
                AppDecor.sectionRule('归档  ·  ${archived.length}'),
                const SizedBox(height: 4),
                for (final m in archived)
                  _SwipeableMissionCard(mission: m, seriesSize: groups[m.seriesId]!.length),
              ],
            ],
          ),
        );
      },
    );
  }
}

class _SwipeableMissionCard extends ConsumerWidget {
  const _SwipeableMissionCard({required this.mission, this.seriesSize = 1});
  final Mission mission;
  final int seriesSize;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Dismissible(
      key: ValueKey(mission.id),
      direction: DismissDirection.endToStart,
      confirmDismiss: (_) async {
        return await showDialog<bool>(
          context: context,
          builder: (ctx) => _DeleteConfirmDialog(codename: mission.codename),
        ) ?? false;
      },
      onDismissed: (_) async {
        try {
          await ref.read(deleteMissionProvider).call(mission.id);
          if (context.mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                backgroundColor: AppTheme.ink,
                content: Text('已销毁档案：${mission.codename}',
                    style: const TextStyle(color: AppTheme.paper)),
                duration: const Duration(seconds: 2),
              ),
            );
          }
        } catch (e) {
          if (context.mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                backgroundColor: AppTheme.redline,
                content: Text('销毁失败：$e', style: const TextStyle(color: AppTheme.paper)),
                duration: const Duration(seconds: 4),
              ),
            );
            ref.invalidate(missionsListProvider);
          }
        }
      },
      background: Container(
        margin: const EdgeInsets.symmetric(vertical: 6),
        padding: const EdgeInsets.only(right: 24),
        alignment: Alignment.centerRight,
        decoration: BoxDecoration(
          color: AppTheme.redline,
          border: Border.all(color: AppTheme.redline, width: 0.8),
        ),
        child: const Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(CupertinoIcons.delete_solid, color: AppTheme.paper, size: 22),
            SizedBox(height: 4),
            Text('销毁档案',
                style: TextStyle(
                  color: AppTheme.paper,
                  fontSize: 11,
                  letterSpacing: 3,
                  fontWeight: FontWeight.w700,
                  fontFamilyFallback: AppTheme.monoFallback,
                )),
          ],
        ),
      ),
      child: _MissionCard(mission: mission, seriesSize: seriesSize),
    );
  }
}

class _DeleteConfirmDialog extends StatelessWidget {
  const _DeleteConfirmDialog({required this.codename});
  final String codename;

  @override
  Widget build(BuildContext context) {
    return Dialog(
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
      backgroundColor: AppTheme.carbon,
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AppDecor.stamp('PURGE', border: AppTheme.redline, color: AppTheme.redline),
            const SizedBox(height: 14),
            Text(
              '销毁「$codename」档案？',
              style: const TextStyle(color: AppTheme.paper, fontSize: 17, fontWeight: FontWeight.w700, letterSpacing: 2),
            ),
            const SizedBox(height: 8),
            const Text(
              '该任务、它产出的所有思考、调用回执、工件柜里的产出，都会从行动局抹掉，无法恢复。',
              style: TextStyle(color: AppTheme.pen, fontSize: 13, height: 1.5),
            ),
            const SizedBox(height: 18),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                OutlinedButton(onPressed: () => Navigator.pop(context, false), child: const Text('保留')),
                const SizedBox(width: 8),
                FilledButton(
                  style: FilledButton.styleFrom(backgroundColor: AppTheme.redline),
                  onPressed: () => Navigator.pop(context, true),
                  child: const Text('销毁'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _MissionCard extends StatelessWidget {
  const _MissionCard({required this.mission, this.seriesSize = 1});
  final Mission mission;
  final int seriesSize;

  @override
  Widget build(BuildContext context) {
    final isTerminal = mission.status.isTerminal;
    final isDossier = seriesSize > 1;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Stack(
        clipBehavior: Clip.hardEdge,
        children: [
          InkWell(
        onTap: () => context.push('/missions/${mission.id}'),
        child: Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: AppTheme.carbon,
            border: Border.all(color: AppTheme.graphite, width: 0.8),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (isDossier)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Row(
                    children: [
                      AppDecor.stamp('DOSSIER', border: AppTheme.amber, color: AppTheme.amber),
                      const SizedBox(width: 8),
                      Text('行动卷宗 · $seriesSize 卷',
                          style: const TextStyle(
                            color: AppTheme.amber,
                            fontSize: 11,
                            letterSpacing: 3,
                            fontFamilyFallback: AppTheme.monoFallback,
                          )),
                    ],
                  ),
                ),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      mission.codename,
                      style: const TextStyle(
                        color: AppTheme.paper,
                        fontSize: 18,
                        fontWeight: FontWeight.w700,
                        letterSpacing: 2,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  // 终态由右下角的大盖章表达状态，这里不再重复
                  if (!isTerminal) _StatusChip(status: mission.status),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                mission.brief,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: AppTheme.pen, fontSize: 13, height: 1.45),
              ),
              const SizedBox(height: 14),
              Row(
                children: [
                  _MetaTag(label: 'TIER', value: mission.tier.label),
                  const SizedBox(width: 12),
                  _MetaTag(label: 'KIT', value: mission.loadout.length.toString()),
                  const SizedBox(width: 12),
                  _MetaTag(
                    label: 'TOKEN',
                    value: '${mission.inputTokens + mission.outputTokens}',
                  ),
                  const Spacer(),
                  Text(
                    DateFormat('MM/dd HH:mm').format(mission.createdAt),
                    style: const TextStyle(
                      color: AppTheme.muted,
                      fontSize: 10,
                      letterSpacing: 2,
                      fontFamilyFallback: AppTheme.monoFallback,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Icon(
                    isTerminal ? CupertinoIcons.chevron_right : CupertinoIcons.dot_radiowaves_left_right,
                    size: 14,
                    color: isTerminal ? AppTheme.muted : AppTheme.redline,
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
          if (isTerminal)
            Positioned(
              right: 12,
              bottom: 28,
              child: IgnorePointer(
                child: MissionStamp(status: mission.status),
              ),
            ),
        ],
      ),
    );
  }
}

/// 任务终态时叠在卡片右下的斜盖章。
/// 仿《使命召唤》接令章戳，按状态切换颜色和文案。
class MissionStamp extends StatelessWidget {
  const MissionStamp({super.key, required this.status});
  final MissionStatus status;

  @override
  Widget build(BuildContext context) {
    String text;
    Color color;
    switch (status) {
      case MissionStatus.done:
        text = 'MISSION COMPLETED';
        color = AppTheme.sage;
        break;
      case MissionStatus.aborted:
        text = 'MISSION ABORTED';
        color = AppTheme.amber;
        break;
      case MissionStatus.error:
        text = 'MISSION FAILED';
        color = AppTheme.redline;
        break;
      default:
        return const SizedBox.shrink();
    }
    return Transform.rotate(
      angle: -math.pi / 16,
      alignment: Alignment.centerRight,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 3),
        decoration: BoxDecoration(
          border: Border.all(color: color.withValues(alpha: 0.85), width: 1.8),
          color: color.withValues(alpha: 0.08),
        ),
        child: Text(
          text,
          style: TextStyle(
            color: color.withValues(alpha: 0.92),
            fontSize: 11,
            fontWeight: FontWeight.w900,
            letterSpacing: 2,
            fontFamilyFallback: AppTheme.monoFallback,
            shadows: [
              Shadow(color: color.withValues(alpha: 0.35), blurRadius: 4),
            ],
          ),
        ),
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});
  final MissionStatus status;

  Color get _color {
    switch (status) {
      case MissionStatus.running:
        return AppTheme.redline;
      case MissionStatus.done:
        return AppTheme.sage;
      case MissionStatus.aborted:
      case MissionStatus.error:
        return AppTheme.amber;
      case MissionStatus.pending:
        return AppTheme.pen;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AppDecor.stamp(status.label, border: _color, color: _color);
  }
}

class _MetaTag extends StatelessWidget {
  const _MetaTag({required this.label, required this.value});
  final String label;
  final String value;
  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Text(
          label,
          style: const TextStyle(
            color: AppTheme.muted,
            fontSize: 9,
            letterSpacing: 2,
            fontWeight: FontWeight.w600,
            fontFamilyFallback: AppTheme.monoFallback,
          ),
        ),
        const SizedBox(width: 4),
        Text(
          value,
          style: const TextStyle(
            color: AppTheme.paper,
            fontSize: 11,
            letterSpacing: 1,
            fontWeight: FontWeight.w600,
            fontFamilyFallback: AppTheme.monoFallback,
          ),
        ),
      ],
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState();
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            AppDecor.stamp('EMPTY', border: AppTheme.pen, color: AppTheme.pen),
            const SizedBox(height: 24),
            const Text(
              '任务簿空白',
              style: TextStyle(
                color: AppTheme.paper,
                fontSize: 20,
                fontWeight: FontWeight.w700,
                letterSpacing: 4,
              ),
            ),
            const SizedBox(height: 10),
            Text(
              '还没派遣过任何行动。\n点右下角红色按钮，写一句你想让代号零完成的任务。',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppTheme.pen),
            ),
          ],
        ),
      ),
    );
  }
}

class _Loading extends StatelessWidget {
  const _Loading();
  @override
  Widget build(BuildContext context) {
    return const Center(
      child: SizedBox(
        height: 28,
        width: 28,
        child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
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
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            AppDecor.stamp('CONNECTION FAILED', border: AppTheme.redline, color: AppTheme.redline),
            const SizedBox(height: 18),
            Text(message, textAlign: TextAlign.center, style: const TextStyle(color: AppTheme.pen, fontSize: 12)),
            const SizedBox(height: 18),
            OutlinedButton(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      ),
    );
  }
}

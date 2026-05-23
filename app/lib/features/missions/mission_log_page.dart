import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../core/theme.dart';
import '../../models/mission.dart';
import '../../providers/auth.dart';
import '../../providers/missions.dart';

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
    if (!auth.isSignedIn) {
      return const _SignInGate();
    }
    return _MissionLogShell(child: _MissionList());
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
    final date = DateFormat('yyyy.MM.dd').format(DateTime.now());
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
            Text('DATE · $date', style: const TextStyle(
              color: AppTheme.pen,
              fontSize: 10,
              letterSpacing: 3,
              fontFamilyFallback: AppTheme.monoFallback,
            )),
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
        final running = items.where((m) => !m.status.isTerminal).toList();
        final archived = items.where((m) => m.status.isTerminal).toList();
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
                for (final m in running) _MissionCard(mission: m),
                const SizedBox(height: 24),
              ],
              if (archived.isNotEmpty) ...[
                AppDecor.sectionRule('归档  ·  ${archived.length}'),
                const SizedBox(height: 4),
                for (final m in archived) _MissionCard(mission: m),
              ],
            ],
          ),
        );
      },
    );
  }
}

class _MissionCard extends StatelessWidget {
  const _MissionCard({required this.mission});
  final Mission mission;

  @override
  Widget build(BuildContext context) {
    final isTerminal = mission.status.isTerminal;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: InkWell(
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
                  _StatusChip(status: mission.status),
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

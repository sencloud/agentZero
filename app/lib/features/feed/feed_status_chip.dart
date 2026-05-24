import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/theme.dart';
import '../../models/feed.dart';
import '../../providers/feed.dart';

/// 任务簿右上角的「事件流」chip。
///
/// 三态颜色：
///   - 绿点 + EVENT FEED · LIVE：worker running + no error
///   - 朱红点 + EVENT FEED · ERR：last_error 非空
///   - 灰点 + EVENT FEED · IDLE：worker 未启动或后端不可达
class FeedStatusChip extends ConsumerWidget {
  const FeedStatusChip({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final statusAsync = ref.watch(feedStatusProvider);
    final st = statusAsync.value;
    final dot = _DotColor.from(st);

    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => context.push('/feed'),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          border: Border.all(color: AppTheme.graphite, width: 0.8),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            _PulsingDot(color: dot.color, pulsing: dot.pulsing),
            const SizedBox(width: 6),
            Text(
              dot.label,
              style: const TextStyle(
                color: AppTheme.pen,
                fontSize: 9.5,
                letterSpacing: 2.5,
                fontFamilyFallback: AppTheme.monoFallback,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _DotColor {
  _DotColor({required this.color, required this.label, required this.pulsing});
  final Color color;
  final String label;
  final bool pulsing;

  factory _DotColor.from(FeedStatus? st) {
    if (st == null) {
      return _DotColor(color: AppTheme.muted, label: '事件流 · IDLE', pulsing: false);
    }
    if (st.lastError.isNotEmpty) {
      return _DotColor(color: AppTheme.redline, label: '事件流 · ERR', pulsing: true);
    }
    if (st.running) {
      return _DotColor(color: const Color(0xFF67D17B), label: '事件流 · LIVE', pulsing: true);
    }
    return _DotColor(color: AppTheme.muted, label: '事件流 · IDLE', pulsing: false);
  }
}

class _PulsingDot extends StatefulWidget {
  const _PulsingDot({required this.color, required this.pulsing});
  final Color color;
  final bool pulsing;

  @override
  State<_PulsingDot> createState() => _PulsingDotState();
}

class _PulsingDotState extends State<_PulsingDot> with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1600),
    );
    if (widget.pulsing) _ctrl.repeat(reverse: true);
  }

  @override
  void didUpdateWidget(covariant _PulsingDot oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.pulsing && !_ctrl.isAnimating) {
      _ctrl.repeat(reverse: true);
    } else if (!widget.pulsing && _ctrl.isAnimating) {
      _ctrl.stop();
      _ctrl.value = 1.0;
    }
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _ctrl,
      builder: (_, _) {
        final t = widget.pulsing ? 0.4 + 0.6 * _ctrl.value : 1.0;
        return Container(
          width: 7,
          height: 7,
          decoration: BoxDecoration(
            color: widget.color.withValues(alpha: t),
            shape: BoxShape.circle,
            boxShadow: widget.pulsing
                ? [BoxShadow(color: widget.color.withValues(alpha: 0.6 * t), blurRadius: 6)]
                : null,
          ),
        );
      },
    );
  }
}

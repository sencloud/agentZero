import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../models/agent.dart';
import '../providers/auth.dart';
import '../providers/install.dart';
import 'agent_icon.dart';

class AgentRow extends ConsumerWidget {
  const AgentRow({super.key, required this.agent, this.rank});
  final Agent agent;
  final int? rank;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return InkWell(
      onTap: () => context.push('/agent/${agent.slug}'),
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 4),
        child: Row(
          children: [
            if (rank != null)
              SizedBox(
                width: 26,
                child: Text('$rank',
                    style: const TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.w700,
                      color: Color(0xFF8E8E93),
                    )),
              ),
            AgentIcon(iconUrl: agent.iconUrl, size: 60),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(agent.name, style: Theme.of(context).textTheme.titleMedium, maxLines: 1, overflow: TextOverflow.ellipsis),
                  const SizedBox(height: 2),
                  Text(agent.tagline, style: Theme.of(context).textTheme.bodyMedium, maxLines: 2, overflow: TextOverflow.ellipsis),
                  const SizedBox(height: 6),
                  Text(agent.categoryName, style: Theme.of(context).textTheme.bodySmall),
                ],
              ),
            ),
            const SizedBox(width: 8),
            _InstallButton(agent: agent),
          ],
        ),
      ),
    );
  }
}

class _InstallButton extends ConsumerStatefulWidget {
  const _InstallButton({required this.agent});
  final Agent agent;
  @override
  ConsumerState<_InstallButton> createState() => _InstallButtonState();
}

class _InstallButtonState extends ConsumerState<_InstallButton> {
  bool _busy = false;

  Future<void> _onTap() async {
    final auth = ref.read(authProvider);
    if (!auth.isSignedIn) {
      _showLoginPrompt();
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

  void _showLoginPrompt() {
    showCupertinoModalPopup(
      context: context,
      builder: (ctx) => CupertinoActionSheet(
        title: const Text('请先登录'),
        message: const Text('登录后即可使用智能体'),
        actions: [
          CupertinoActionSheetAction(
            onPressed: () {
              Navigator.pop(ctx);
              GoRouter.of(context).push('/sign-in');
            },
            child: const Text('使用 Apple 登录'),
          ),
        ],
        cancelButton: CupertinoActionSheetAction(
          isDefaultAction: true,
          onPressed: () => Navigator.pop(ctx),
          child: const Text('取消'),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final installed = widget.agent.installed;
    final label = installed ? '打开' : (widget.agent.isFree ? '获取' : '￥${(widget.agent.priceCents / 100).toStringAsFixed(2)}');
    final onPressed = _busy
        ? null
        : (installed ? () => context.push('/agent/${widget.agent.slug}/chat') : _onTap);
    return SizedBox(
      height: 32,
      child: CupertinoButton(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        borderRadius: BorderRadius.circular(20),
        color: const Color(0xFFE5E5EA),
        onPressed: onPressed,
        child: _busy
            ? const CupertinoActivityIndicator(radius: 8)
            : Text(label,
                style: const TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                  color: Color(0xFF0A84FF),
                )),
      ),
    );
  }
}

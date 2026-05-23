import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../providers/auth.dart';
import '../../providers/catalog.dart';
import '../../widgets/agent_row.dart';

class ProfilePage extends ConsumerWidget {
  const ProfilePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
          children: [
            Text('我的', style: Theme.of(context).textTheme.displayLarge),
            const SizedBox(height: 16),
            if (!auth.isSignedIn)
              _SignInCard(onTap: () => context.push('/sign-in'))
            else
              _AccountCard(name: auth.user!.nickname, email: auth.user!.email),
            const SizedBox(height: 28),
            if (auth.isSignedIn) ...[
              Text('我的智能体', style: Theme.of(context).textTheme.headlineMedium),
              const SizedBox(height: 8),
              _InstalledList(),
              const SizedBox(height: 28),
            ],
            Text('账户', style: Theme.of(context).textTheme.headlineMedium),
            const SizedBox(height: 8),
            Container(
              decoration: BoxDecoration(
                color: Theme.of(context).cardColor,
                borderRadius: BorderRadius.circular(14),
              ),
              child: Column(
                children: [
                  _SettingsTile(label: '订阅与购买', icon: CupertinoIcons.bag),
                  const Divider(height: 1, indent: 56),
                  _SettingsTile(label: '通知设置', icon: CupertinoIcons.bell),
                  const Divider(height: 1, indent: 56),
                  _SettingsTile(label: '隐私', icon: CupertinoIcons.lock),
                  const Divider(height: 1, indent: 56),
                  _SettingsTile(label: '关于 AgentZero', icon: CupertinoIcons.info),
                  if (auth.isSignedIn) ...[
                    const Divider(height: 1, indent: 56),
                    _SettingsTile(
                      label: '退出登录',
                      icon: CupertinoIcons.square_arrow_right,
                      destructive: true,
                      onTap: () => ref.read(authProvider.notifier).signOut(),
                    ),
                  ]
                ],
              ),
            ),
            const SizedBox(height: 24),
            const Center(
              child: Text(
                'AgentZero · 0.1.0 (Beta)',
                style: TextStyle(color: Color(0xFF8E8E93), fontSize: 12),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SignInCard extends StatelessWidget {
  const _SignInCard({required this.onTap});
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(18),
        decoration: BoxDecoration(
          color: Theme.of(context).cardColor,
          borderRadius: BorderRadius.circular(16),
        ),
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: const BoxDecoration(
                color: Color(0xFFE5E5EA),
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: const Icon(CupertinoIcons.person, color: Color(0xFF8E8E93), size: 32),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('登录 AgentZero', style: Theme.of(context).textTheme.titleLarge),
                  const SizedBox(height: 2),
                  Text('使用 Apple 账户继续，同步你的智能体与对话',
                      style: Theme.of(context).textTheme.bodyMedium),
                ],
              ),
            ),
            const Icon(CupertinoIcons.chevron_right, size: 14, color: Color(0xFF8E8E93)),
          ],
        ),
      ),
    );
  }
}

class _AccountCard extends StatelessWidget {
  const _AccountCard({required this.name, required this.email});
  final String name;
  final String email;
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: Theme.of(context).cardColor,
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        children: [
          Container(
            width: 56,
            height: 56,
            decoration: const BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [Color(0xFF0A84FF), Color(0xFFBF5AF2)],
              ),
              shape: BoxShape.circle,
            ),
            alignment: Alignment.center,
            child: Text(
              (name.isNotEmpty ? name.characters.first : 'A').toUpperCase(),
              style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w800, fontSize: 24),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: Theme.of(context).textTheme.titleLarge),
                const SizedBox(height: 2),
                Text(email.isEmpty ? 'Apple ID 已绑定' : email,
                    style: Theme.of(context).textTheme.bodyMedium),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _InstalledList extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final installed = ref.watch(installedAgentsProvider);
    return installed.when(
      loading: () => const Padding(
        padding: EdgeInsets.symmetric(vertical: 32),
        child: Center(child: CupertinoActivityIndicator()),
      ),
      error: (e, _) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 16),
        child: Text('加载失败：$e', style: Theme.of(context).textTheme.bodyMedium),
      ),
      data: (items) {
        if (items.isEmpty) {
          return Container(
            padding: const EdgeInsets.symmetric(vertical: 32),
            alignment: Alignment.center,
            child: Text('还没有安装智能体，去「应用」逛逛吧',
                style: Theme.of(context).textTheme.bodyMedium),
          );
        }
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          decoration: BoxDecoration(
            color: Theme.of(context).cardColor,
            borderRadius: BorderRadius.circular(14),
          ),
          child: Column(
            children: [
              for (var i = 0; i < items.length; i++) ...[
                AgentRow(agent: items[i]),
                if (i != items.length - 1) const Divider(indent: 70),
              ]
            ],
          ),
        );
      },
    );
  }
}

class _SettingsTile extends StatelessWidget {
  const _SettingsTile({
    required this.label,
    required this.icon,
    this.destructive = false,
    this.onTap,
  });
  final String label;
  final IconData icon;
  final bool destructive;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final color = destructive ? const Color(0xFFFF3B30) : Theme.of(context).colorScheme.primary;
    return ListTile(
      onTap: onTap,
      leading: Container(
        width: 32,
        height: 32,
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(8),
        ),
        alignment: Alignment.center,
        child: Icon(icon, color: color, size: 18),
      ),
      title: Text(label, style: Theme.of(context).textTheme.titleMedium),
      trailing: destructive ? null : const Icon(CupertinoIcons.chevron_right, size: 14, color: Color(0xFF8E8E93)),
    );
  }
}

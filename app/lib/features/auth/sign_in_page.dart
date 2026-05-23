import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';

import '../../core/theme.dart';
import '../../providers/auth.dart';

/// 登录页：机密文档风。
///
/// 没有任何"市场化"语言；只保留任务前的身份验证仪式感。
class SignInPage extends ConsumerWidget {
  const SignInPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);

    ref.listen(authProvider, (prev, next) {
      if (next.isSignedIn) {
        context.go('/');
      }
    });

    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(32, 24, 32, 32),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  AppDecor.stamp('CLASSIFIED', border: AppTheme.redline, color: AppTheme.redline),
                  Text('FILE · 00-AZ', style: Theme.of(context).textTheme.labelMedium),
                ],
              ),
              const SizedBox(height: 36),
              Container(
                width: 104,
                height: 104,
                decoration: BoxDecoration(
                  border: Border.all(color: AppTheme.paper, width: 1),
                ),
                child: ClipRRect(
                  borderRadius: BorderRadius.zero,
                  child: Image.asset(
                    'assets/branding/app_icon.png',
                    fit: BoxFit.cover,
                  ),
                ),
              ),
              const SizedBox(height: 24),
              const Text(
                '代号零',
                style: TextStyle(
                  color: AppTheme.paper,
                  fontSize: 38,
                  fontWeight: FontWeight.w800,
                  letterSpacing: 10,
                ),
              ),
              const SizedBox(height: 6),
              const Text(
                'AGENT  ·  ZERO',
                style: TextStyle(
                  color: AppTheme.redline,
                  fontSize: 11,
                  letterSpacing: 8,
                  fontWeight: FontWeight.w700,
                  fontFamilyFallback: AppTheme.monoFallback,
                ),
              ),
              const SizedBox(height: 36),
              AppDecor.sectionRule('入职手续'),
              const SizedBox(height: 16),
              Text(
                '本系统仅对登记在册的特工开放。请用你的 Apple 身份完成接入；'
                '完成后会自动颁发行动局编号。',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppTheme.pen),
              ),
              const Spacer(),
              if (auth.error != null) ...[
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(border: Border.all(color: AppTheme.redline)),
                  child: Row(
                    children: [
                      const Icon(CupertinoIcons.exclamationmark_triangle, color: AppTheme.redline, size: 16),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(auth.error!,
                            style: const TextStyle(color: AppTheme.redline, fontSize: 12, letterSpacing: 1)),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 12),
              ],
              SizedBox(
                width: double.infinity,
                child: SignInWithAppleButton(
                  onPressed: () => ref.read(authProvider.notifier).signInWithApple(),
                  style: SignInWithAppleButtonStyle.white,
                  height: 52,
                  borderRadius: BorderRadius.zero,
                ),
              ),
              const SizedBox(height: 14),
              Text(
                '我们只用 Apple 身份标识做特工编号，不读取其他个人信息。',
                style: Theme.of(context).textTheme.labelSmall,
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

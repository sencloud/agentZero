import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';

import '../../providers/auth.dart';

class SignInPage extends ConsumerWidget {
  const SignInPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);

    ref.listen(authProvider, (prev, next) {
      if (next.isSignedIn) {
        context.go('/profile');
      }
    });

    return Scaffold(
      appBar: AppBar(title: const Text(''), elevation: 0),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(24, 12, 24, 32),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 84,
                height: 84,
                decoration: BoxDecoration(
                  gradient: const LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: [Color(0xFF0A84FF), Color(0xFFBF5AF2)],
                  ),
                  borderRadius: BorderRadius.circular(20),
                ),
                alignment: Alignment.center,
                child: const Icon(CupertinoIcons.sparkles, size: 44, color: Colors.white),
              ),
              const SizedBox(height: 28),
              Text('欢迎来到 AgentZero', style: Theme.of(context).textTheme.displayMedium),
              const SizedBox(height: 8),
              Text(
                '挑选你的第一位智能体伙伴，让 AI 帮你写作、写代码、整理生活。',
                style: Theme.of(context).textTheme.bodyLarge,
              ),
              const Spacer(),
              if (auth.error != null) ...[
                Text(auth.error!, style: const TextStyle(color: Color(0xFFFF3B30))),
                const SizedBox(height: 12),
              ],
              SizedBox(
                width: double.infinity,
                child: SignInWithAppleButton(
                  onPressed: () => ref.read(authProvider.notifier).signInWithApple(),
                  style: SignInWithAppleButtonStyle.black,
                  height: 52,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                '我们仅会使用你的 Apple ID 标识来同步智能体，不会发送营销邮件。',
                style: Theme.of(context).textTheme.bodySmall,
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

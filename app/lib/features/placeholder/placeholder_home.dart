import 'package:flutter/material.dart';

/// M0 阶段的占位主屏。
///
/// 真正的「任务簿 / 行动现场 / 装备柜」三屏会在 M3 阶段
/// 替换掉这个文件。这里只保证应用能正常启动、能跑通 CI。
class PlaceholderHome extends StatelessWidget {
  const PlaceholderHome({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF1A1A1A),
      body: SafeArea(
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
                decoration: BoxDecoration(
                  border: Border.all(color: const Color(0xFFEDEAE3), width: 1),
                ),
                child: const Text(
                  'CLASSIFIED',
                  style: TextStyle(
                    color: Color(0xFFEDEAE3),
                    fontSize: 11,
                    letterSpacing: 4,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const SizedBox(height: 32),
              const Text(
                '代号零',
                style: TextStyle(
                  color: Color(0xFFEDEAE3),
                  fontSize: 40,
                  fontWeight: FontWeight.w800,
                  letterSpacing: 6,
                ),
              ),
              const SizedBox(height: 8),
              const Text(
                'AGENT  ·  ZERO',
                style: TextStyle(
                  color: Color(0xFFC8362E),
                  fontSize: 12,
                  letterSpacing: 8,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 48),
              const Text(
                '系统组装中  ·  待命',
                style: TextStyle(
                  color: Color(0xFF7A7A7A),
                  fontSize: 13,
                  letterSpacing: 1,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

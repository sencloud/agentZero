import 'package:flutter/material.dart';

/// AgentZero 的视觉语言：「机密文档」风。
///
/// 色板的意图：
///   ink/carbon/graphite — 暗背景三档黑，避免一片死黑
///   paper/pen           — 仿旧档案纸的灰白，比 #FFFFFF 更"有质感"
///   redline             — 朱红，仅用于派遣/撤离/警告/接令章戳这类"高扭矩"事件
///   amber               — 琥珀，用于思考块、提示信息、提示型徽章
///
/// 字号策略：
///   display/title 用比常规略大的字 + 加粗，模拟印刷标题感
///   行动事件流（思考、tool_call、tool_result、入柜）一律走等宽，强化"系统日志"质感
class AppTheme {
  // ===== Palette =====
  static const ink = Color(0xFF0E0E0E);      // 主背景
  static const carbon = Color(0xFF161616);   // 二级面 / 卡片
  static const graphite = Color(0xFF252525); // 边框 / 分割线 / 三级面
  static const paper = Color(0xFFEDEAE3);    // 主文字 / 章戳描边
  static const pen = Color(0xFFB6B0A4);      // 次要文字
  static const muted = Color(0xFF6E6A60);    // 最弱文字 / placeholder

  static const redline = Color(0xFFC8362E);  // 派遣 / 撤离 / 警告
  static const redlineDim = Color(0xFF7A2520);
  static const amber = Color(0xFFC99A4B);    // 思考块 / 提示
  static const amberDim = Color(0xFF6F5424);
  static const sage = Color(0xFF6FA37A);     // 入柜成功 / "task_done"

  // ===== Build dark theme =====
  static ThemeData dark() {
    final base = ThemeData.dark(useMaterial3: true);
    final scheme = base.colorScheme.copyWith(
      brightness: Brightness.dark,
      primary: paper,
      onPrimary: ink,
      secondary: amber,
      onSecondary: ink,
      tertiary: redline,
      surface: carbon,
      onSurface: paper,
      surfaceContainerHighest: graphite,
      error: redline,
      onError: paper,
      outline: graphite,
    );

    return base.copyWith(
      colorScheme: scheme,
      scaffoldBackgroundColor: ink,
      canvasColor: ink,
      cardColor: carbon,
      dividerColor: graphite,

      textTheme: _buildTextTheme(base.textTheme),
      primaryTextTheme: _buildTextTheme(base.primaryTextTheme),

      appBarTheme: const AppBarTheme(
        backgroundColor: ink,
        elevation: 0,
        scrolledUnderElevation: 0,
        foregroundColor: paper,
        centerTitle: false,
        titleTextStyle: TextStyle(
          color: paper,
          fontSize: 15,
          fontWeight: FontWeight.w600,
          letterSpacing: 2,
          fontFamilyFallback: monoFallback,
        ),
        iconTheme: IconThemeData(color: paper),
      ),

      dividerTheme: const DividerThemeData(
        color: graphite,
        thickness: 0.5,
        space: 0,
      ),

      iconTheme: const IconThemeData(color: paper, size: 18),

      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: redline,
          foregroundColor: paper,
          padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 14),
          shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
          textStyle: const TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w700,
            letterSpacing: 4,
            fontFamilyFallback: monoFallback,
          ),
        ),
      ),

      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: paper,
          side: const BorderSide(color: paper, width: 1),
          padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
          shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
          textStyle: const TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            letterSpacing: 4,
            fontFamilyFallback: monoFallback,
          ),
        ),
      ),

      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: amber,
          textStyle: const TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            letterSpacing: 2,
          ),
        ),
      ),

      inputDecorationTheme: const InputDecorationTheme(
        filled: true,
        fillColor: carbon,
        hintStyle: TextStyle(color: muted, fontSize: 14),
        contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.zero,
          borderSide: BorderSide(color: graphite),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.zero,
          borderSide: BorderSide(color: graphite),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.zero,
          borderSide: BorderSide(color: paper, width: 1),
        ),
      ),

      progressIndicatorTheme: const ProgressIndicatorThemeData(
        color: paper,
        linearTrackColor: graphite,
      ),
    );
  }

  /// light() 给系统自动深浅切换用；视觉风格是"黑底白字"特工感的"反相版"，
  /// 但默认产品永远走 dark；我们保留 light 是为了 Apple App Store 截屏需要。
  static ThemeData light() => dark();

  // ===== Text theme =====
  static TextTheme _buildTextTheme(TextTheme base) {
    const display = TextStyle(
      color: paper,
      fontSize: 38,
      fontWeight: FontWeight.w800,
      letterSpacing: 8,
      height: 1.05,
    );
    const headline = TextStyle(
      color: paper,
      fontSize: 22,
      fontWeight: FontWeight.w700,
      letterSpacing: 4,
    );
    const title = TextStyle(
      color: paper,
      fontSize: 15,
      fontWeight: FontWeight.w600,
      letterSpacing: 2,
    );
    const subtitle = TextStyle(
      color: pen,
      fontSize: 13,
      fontWeight: FontWeight.w400,
      letterSpacing: 0.5,
      height: 1.5,
    );
    const body = TextStyle(
      color: paper,
      fontSize: 15,
      fontWeight: FontWeight.w400,
      height: 1.55,
    );
    const caption = TextStyle(
      color: muted,
      fontSize: 11,
      fontWeight: FontWeight.w500,
      letterSpacing: 3,
    );

    return base.copyWith(
      displayLarge: display,
      displayMedium: display.copyWith(fontSize: 28, letterSpacing: 6),
      displaySmall: display.copyWith(fontSize: 22, letterSpacing: 4),
      headlineLarge: headline,
      headlineMedium: headline.copyWith(fontSize: 18),
      headlineSmall: headline.copyWith(fontSize: 16, letterSpacing: 3),
      titleLarge: title,
      titleMedium: title.copyWith(fontSize: 13, letterSpacing: 1),
      titleSmall: title.copyWith(fontSize: 12, letterSpacing: 1),
      bodyLarge: body,
      bodyMedium: body.copyWith(fontSize: 13),
      bodySmall: subtitle,
      labelLarge: caption.copyWith(fontSize: 12),
      labelMedium: caption,
      labelSmall: caption.copyWith(fontSize: 10),
    );
  }

  // ===== Mono (事件流 / 时间戳 / 代号 / 状态条) =====
  //
  // iOS 默认 system monospace 是 SF Mono；Android/Web 兜底用 Menlo / Consolas。
  // 用 fontFamilyFallback 而不是 fontFamily，是为了让 SF 系列优先。
  static const List<String> monoFallback = <String>[
    'SF Mono',
    'Menlo',
    'Consolas',
    'Roboto Mono',
    'monospace',
  ];

  static const TextStyle monoTitle = TextStyle(
    color: paper,
    fontSize: 12,
    fontWeight: FontWeight.w600,
    letterSpacing: 3,
    fontFamilyFallback: monoFallback,
  );

  static const TextStyle monoEvent = TextStyle(
    color: paper,
    fontSize: 13,
    height: 1.55,
    fontFamilyFallback: monoFallback,
  );

  static const TextStyle monoDim = TextStyle(
    color: muted,
    fontSize: 12,
    height: 1.5,
    fontFamilyFallback: monoFallback,
  );

  static const TextStyle monoAccent = TextStyle(
    color: amber,
    fontSize: 13,
    height: 1.55,
    fontFamilyFallback: monoFallback,
  );

  static const TextStyle monoDanger = TextStyle(
    color: redline,
    fontSize: 13,
    height: 1.55,
    fontWeight: FontWeight.w600,
    fontFamilyFallback: monoFallback,
  );
}

/// 仅用于贴章戳/状态条等装饰元素的小工具盒。
class AppDecor {
  /// 文档章戳：方框 + 等宽全大写文字。eg. "CLASSIFIED"、"DISPATCHED"
  static Widget stamp(String text, {Color border = AppTheme.paper, Color color = AppTheme.paper}) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(border: Border.all(color: border, width: 1)),
      child: Text(
        text.toUpperCase(),
        style: TextStyle(
          color: color,
          fontSize: 10,
          letterSpacing: 4,
          fontWeight: FontWeight.w700,
          fontFamilyFallback: AppTheme.monoFallback,
        ),
      ),
    );
  }

  /// 一根代码注释式横线 + 文字：「── 标题 ──」
  static Widget sectionRule(String title) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          const Expanded(child: Divider(color: AppTheme.graphite)),
          const SizedBox(width: 12),
          Text(
            title.toUpperCase(),
            style: const TextStyle(
              color: AppTheme.pen,
              fontSize: 10,
              letterSpacing: 4,
              fontWeight: FontWeight.w600,
              fontFamilyFallback: AppTheme.monoFallback,
            ),
          ),
          const SizedBox(width: 12),
          const Expanded(child: Divider(color: AppTheme.graphite)),
        ],
      ),
    );
  }
}

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

class AppTheme {
  static const _systemBlue = Color(0xFF0A84FF);

  static ThemeData light() {
    final base = ThemeData.light(useMaterial3: true);
    return base.copyWith(
      colorScheme: ColorScheme.fromSeed(
        seedColor: _systemBlue,
        brightness: Brightness.light,
      ),
      scaffoldBackgroundColor: const Color(0xFFF2F2F7),
      textTheme: _textTheme(base.textTheme, Brightness.light),
      appBarTheme: const AppBarTheme(
        backgroundColor: Color(0xFFF2F2F7),
        elevation: 0,
        scrolledUnderElevation: 0,
        foregroundColor: Color(0xFF1C1C1E),
        titleTextStyle: TextStyle(
          color: Color(0xFF1C1C1E),
          fontSize: 17,
          fontWeight: FontWeight.w600,
        ),
      ),
      dividerTheme: const DividerThemeData(
        color: Color(0xFFE5E5EA),
        thickness: 0.5,
        space: 0,
      ),
      bottomNavigationBarTheme: const BottomNavigationBarThemeData(
        backgroundColor: Color(0xFFF9F9F9),
        selectedItemColor: _systemBlue,
        unselectedItemColor: Color(0xFF8E8E93),
        showUnselectedLabels: true,
      ),
    );
  }

  static ThemeData dark() {
    final base = ThemeData.dark(useMaterial3: true);
    return base.copyWith(
      colorScheme: ColorScheme.fromSeed(
        seedColor: _systemBlue,
        brightness: Brightness.dark,
      ),
      scaffoldBackgroundColor: const Color(0xFF000000),
      textTheme: _textTheme(base.textTheme, Brightness.dark),
      appBarTheme: const AppBarTheme(
        backgroundColor: Color(0xFF000000),
        elevation: 0,
        scrolledUnderElevation: 0,
        foregroundColor: CupertinoColors.white,
        titleTextStyle: TextStyle(
          color: CupertinoColors.white,
          fontSize: 17,
          fontWeight: FontWeight.w600,
        ),
      ),
      dividerTheme: const DividerThemeData(
        color: Color(0xFF2C2C2E),
        thickness: 0.5,
        space: 0,
      ),
      bottomNavigationBarTheme: const BottomNavigationBarThemeData(
        backgroundColor: Color(0xFF111113),
        selectedItemColor: _systemBlue,
        unselectedItemColor: Color(0xFF8E8E93),
        showUnselectedLabels: true,
      ),
    );
  }

  static TextTheme _textTheme(TextTheme base, Brightness b) {
    final primary = b == Brightness.dark ? CupertinoColors.white : const Color(0xFF1C1C1E);
    final secondary = b == Brightness.dark ? const Color(0xFFAEAEB2) : const Color(0xFF6E6E73);
    return base.copyWith(
      displayLarge: TextStyle(color: primary, fontSize: 34, fontWeight: FontWeight.w800, letterSpacing: -1.2),
      displayMedium: TextStyle(color: primary, fontSize: 28, fontWeight: FontWeight.w800, letterSpacing: -0.8),
      displaySmall: TextStyle(color: primary, fontSize: 22, fontWeight: FontWeight.w700, letterSpacing: -0.4),
      headlineMedium: TextStyle(color: primary, fontSize: 20, fontWeight: FontWeight.w700),
      titleLarge: TextStyle(color: primary, fontSize: 17, fontWeight: FontWeight.w600),
      titleMedium: TextStyle(color: primary, fontSize: 15, fontWeight: FontWeight.w600),
      bodyLarge: TextStyle(color: primary, fontSize: 15, fontWeight: FontWeight.w400, height: 1.4),
      bodyMedium: TextStyle(color: secondary, fontSize: 13, fontWeight: FontWeight.w400, height: 1.3),
      bodySmall: TextStyle(color: secondary, fontSize: 12, fontWeight: FontWeight.w400),
      labelLarge: TextStyle(color: primary, fontSize: 14, fontWeight: FontWeight.w600),
    );
  }
}

class AppColors {
  static const eyebrow = Color(0xFF8E8E93);
  static const tagBackgroundLight = Color(0xFFF2F2F7);
  static const tagBackgroundDark = Color(0xFF1C1C1E);
  static const cardLight = Colors.white;
  static const cardDark = Color(0xFF1C1C1E);
  static const sectionDividerLight = Color(0xFFE5E5EA);
  static const sectionDividerDark = Color(0xFF2C2C2E);
}

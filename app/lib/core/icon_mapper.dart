import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

/// 解析后端约定的 `ag-icon://<hexColor>/<sfsymbolName>` URI。
class AgentIconSpec {
  AgentIconSpec({required this.color, required this.icon});
  final Color color;
  final IconData icon;

  static AgentIconSpec parse(String raw) {
    final defaultColor = const Color(0xFF0A84FF);
    final defaultIcon = CupertinoIcons.sparkles;
    try {
      final uri = Uri.parse(raw);
      Color color = defaultColor;
      if (uri.host.isNotEmpty) {
        color = Color(int.parse('FF${uri.host.toUpperCase()}', radix: 16));
      }
      final name = uri.path.replaceFirst(RegExp(r'^/'), '');
      final icon = _symbolMap[name] ?? defaultIcon;
      return AgentIconSpec(color: color, icon: icon);
    } catch (_) {
      return AgentIconSpec(color: defaultColor, icon: defaultIcon);
    }
  }
}

// SF Symbol 名字 → Flutter IconData。
// 没有完全对应的就找一个语义最接近的图标。
const Map<String, IconData> _symbolMap = {
  'pencil.tip.crop.circle.badge.plus': CupertinoIcons.pencil_circle_fill,
  'chevron.left.forwardslash.chevron.right': CupertinoIcons.chevron_left_slash_chevron_right,
  'heart.text.square.fill': CupertinoIcons.heart_circle_fill,
  'graduationcap.fill': Icons.school_rounded,
  'paintpalette.fill': Icons.palette_rounded,
  'terminal.fill': Icons.terminal_rounded,
  'globe.asia.australia.fill': CupertinoIcons.globe,
  'fork.knife': Icons.restaurant_rounded,
  'scroll.fill': Icons.menu_book_rounded,
  'doc.text.magnifyingglass': CupertinoIcons.doc_text_search,
  'book.fill': CupertinoIcons.book_fill,
  'figure.run': Icons.directions_run_rounded,
  'bolt.fill': CupertinoIcons.bolt_fill,
  'gamecontroller.fill': Icons.sports_esports_rounded,
  'leaf.fill': Icons.eco_rounded,
  'books.vertical.fill': Icons.menu_book_rounded,
};

class CategoryIconSpec {
  static IconData iconFor(String name) {
    return _symbolMap[name] ?? CupertinoIcons.square_grid_2x2_fill;
  }

  static Color colorFor(String hex) {
    final clean = hex.replaceFirst('#', '');
    return Color(int.parse('FF$clean', radix: 16));
  }
}

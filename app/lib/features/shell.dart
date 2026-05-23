import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class AppShell extends StatelessWidget {
  const AppShell({super.key, required this.navigationShell});
  final StatefulNavigationShell navigationShell;

  static const _items = <_TabItem>[
    _TabItem(label: '今日', icon: CupertinoIcons.calendar_today),
    _TabItem(label: '应用', icon: CupertinoIcons.square_grid_2x2),
    _TabItem(label: '搜索', icon: CupertinoIcons.search),
    _TabItem(label: '我的', icon: CupertinoIcons.person_circle),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: navigationShell,
      bottomNavigationBar: BottomNavigationBar(
        type: BottomNavigationBarType.fixed,
        currentIndex: navigationShell.currentIndex,
        onTap: (i) => navigationShell.goBranch(i, initialLocation: i == navigationShell.currentIndex),
        items: [
          for (final t in _items)
            BottomNavigationBarItem(icon: Icon(t.icon), label: t.label),
        ],
      ),
    );
  }
}

class _TabItem {
  const _TabItem({required this.label, required this.icon});
  final String label;
  final IconData icon;
}

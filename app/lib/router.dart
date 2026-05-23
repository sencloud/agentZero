import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'features/placeholder/placeholder_home.dart';

final _rootKey = GlobalKey<NavigatorState>();

/// M0 阶段的最小路由表。
///
/// 真正的「/missions / /missions/new / /missions/:id」等路由
/// 会在 M3 阶段加入。这里只保证应用能启动到一个占位首页。
GoRouter buildRouter() {
  return GoRouter(
    navigatorKey: _rootKey,
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => const PlaceholderHome(),
      ),
    ],
  );
}

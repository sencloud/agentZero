import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'features/apps/apps_page.dart';
import 'features/apps/category_page.dart';
import 'features/chat/chat_page.dart';
import 'features/detail/agent_detail_page.dart';
import 'features/profile/profile_page.dart';
import 'features/profile/sign_in_page.dart';
import 'features/search/search_page.dart';
import 'features/shell.dart';
import 'features/today/today_page.dart';

final _rootKey = GlobalKey<NavigatorState>();

GoRouter buildRouter() {
  return GoRouter(
    navigatorKey: _rootKey,
    initialLocation: '/today',
    routes: [
      StatefulShellRoute.indexedStack(
        builder: (context, state, navigationShell) => AppShell(navigationShell: navigationShell),
        branches: [
          StatefulShellBranch(routes: [
            GoRoute(path: '/today', pageBuilder: (_, _) => const NoTransitionPage(child: TodayPage())),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/apps', pageBuilder: (_, _) => const NoTransitionPage(child: AppsPage())),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/search', pageBuilder: (_, _) => const NoTransitionPage(child: SearchPage())),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/profile', pageBuilder: (_, _) => const NoTransitionPage(child: ProfilePage())),
          ]),
        ],
      ),
      GoRoute(
        parentNavigatorKey: _rootKey,
        path: '/agent/:slug',
        builder: (_, state) => AgentDetailPage(slug: state.pathParameters['slug']!),
      ),
      GoRoute(
        parentNavigatorKey: _rootKey,
        path: '/agent/:slug/chat',
        builder: (_, state) => ChatPage(slug: state.pathParameters['slug']!),
      ),
      GoRoute(
        parentNavigatorKey: _rootKey,
        path: '/category/:slug',
        builder: (_, state) => CategoryPage(
          slug: state.pathParameters['slug']!,
          name: state.uri.queryParameters['name'] ?? '',
        ),
      ),
      GoRoute(
        parentNavigatorKey: _rootKey,
        path: '/sign-in',
        builder: (_, _) => const SignInPage(),
      ),
    ],
  );
}

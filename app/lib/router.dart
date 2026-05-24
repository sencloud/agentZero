import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'features/auth/sign_in_page.dart';
import 'features/missions/artifact_viewer_page.dart';
import 'features/missions/dispatch_page.dart';
import 'features/missions/mission_log_page.dart';
import 'features/missions/operation_room_page.dart';

final _rootKey = GlobalKey<NavigatorState>();

GoRouter buildRouter() {
  return GoRouter(
    navigatorKey: _rootKey,
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => const MissionLogPage(),
      ),
      GoRoute(
        path: '/sign-in',
        builder: (_, _) => const SignInPage(),
      ),
      GoRoute(
        path: '/missions/new',
        builder: (_, state) => DispatchPage(parentId: state.uri.queryParameters['parent_id']),
      ),
      GoRoute(
        path: '/missions/:id',
        builder: (_, state) => OperationRoomPage(missionId: state.pathParameters['id']!),
      ),
      GoRoute(
        path: '/missions/:id/artifacts/:aid',
        builder: (_, state) {
          final missionId = state.pathParameters['id']!;
          final aid = int.tryParse(state.pathParameters['aid'] ?? '') ?? 0;
          final name = state.uri.queryParameters['name'] ?? '工件';
          final mime = state.uri.queryParameters['mime'] ?? '';
          return ArtifactViewerPage(
            missionId: missionId,
            artifactId: aid,
            artifactName: name,
            artifactMime: mime,
          );
        },
      ),
    ],
  );
}

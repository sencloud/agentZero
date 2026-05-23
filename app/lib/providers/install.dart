import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import 'catalog.dart';

class InstallController {
  InstallController(this._ref);
  final Ref _ref;

  Future<void> install(String slug) async {
    final api = _ref.read(apiClientProvider);
    await api.dio.post('/agents/$slug/install');
    _ref.invalidate(agentDetailProvider(slug));
    _ref.invalidate(installedAgentsProvider);
  }

  Future<void> uninstall(String slug) async {
    final api = _ref.read(apiClientProvider);
    await api.dio.delete('/agents/$slug/install');
    _ref.invalidate(agentDetailProvider(slug));
    _ref.invalidate(installedAgentsProvider);
  }
}

final installControllerProvider = Provider<InstallController>((ref) => InstallController(ref));

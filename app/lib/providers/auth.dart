import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';

import '../core/api_client.dart';
import '../core/storage.dart';
import '../models/user.dart';

class AuthState {
  AuthState({this.user, this.loading = false, this.error});
  final AppUser? user;
  final bool loading;
  final String? error;

  bool get isSignedIn => user != null;

  AuthState copyWith({AppUser? user, bool? loading, String? error, bool clearUser = false, bool clearError = false}) {
    return AuthState(
      user: clearUser ? null : (user ?? this.user),
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

class AuthNotifier extends StateNotifier<AuthState> {
  AuthNotifier(this._ref) : super(AuthState()) {
    _bootstrap();
  }

  final Ref _ref;

  Future<void> _bootstrap() async {
    final api = _ref.read(apiClientProvider);
    final storage = _ref.read(tokenStorageProvider);
    if (storage.read() == null) return;
    try {
      final r = await api.dio.get('/me');
      state = state.copyWith(user: AppUser.fromJson(r.data as Map<String, dynamic>));
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        await storage.clear();
      }
    } catch (_) {}
  }

  Future<void> signInWithApple() async {
    state = state.copyWith(loading: true, clearError: true);
    try {
      final credential = await SignInWithApple.getAppleIDCredential(
        scopes: [
          AppleIDAuthorizationScopes.email,
          AppleIDAuthorizationScopes.fullName,
        ],
      );
      final fullName = [credential.givenName, credential.familyName]
          .whereType<String>()
          .where((s) => s.isNotEmpty)
          .join(' ');
      final api = _ref.read(apiClientProvider);
      final storage = _ref.read(tokenStorageProvider);
      final r = await api.dio.post('/auth/apple', data: {
        'identity_token': credential.identityToken,
        'full_name': fullName,
        'email': credential.email ?? '',
      });
      final data = r.data as Map<String, dynamic>;
      await storage.save(data['token'] as String);
      state = AuthState(user: AppUser.fromJson(data['user'] as Map<String, dynamic>));
    } catch (e) {
      if (kDebugMode) {
        debugPrint('apple sign in failed: $e');
      }
      state = state.copyWith(loading: false, error: '登录失败，请重试');
    }
  }

  Future<void> signOut() async {
    await _ref.read(tokenStorageProvider).clear();
    state = AuthState();
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(ref);
});

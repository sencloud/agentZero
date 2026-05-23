import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';

import '../core/api_client.dart';
import '../core/storage.dart';
import '../models/user.dart';

class AuthState {
  AuthState({this.user, this.loading = false, this.bootstrapped = false, this.error});
  final AppUser? user;

  /// signIn 过程中（点了按钮、等待 Apple/服务器返回）。
  final bool loading;

  /// 启动时的"看看本地有没有 token 并验过身份"流程是否完成。
  /// false → UI 应当显示 splash / loading，而非登录 gate，避免登录态闪屏。
  final bool bootstrapped;

  final String? error;

  bool get isSignedIn => user != null;

  AuthState copyWith({
    AppUser? user,
    bool? loading,
    bool? bootstrapped,
    String? error,
    bool clearUser = false,
    bool clearError = false,
  }) {
    return AuthState(
      user: clearUser ? null : (user ?? this.user),
      loading: loading ?? this.loading,
      bootstrapped: bootstrapped ?? this.bootstrapped,
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
    if (storage.read() == null) {
      state = state.copyWith(bootstrapped: true);
      return;
    }
    try {
      final r = await api.dio.get('/me');
      state = state.copyWith(
        user: AppUser.fromJson(r.data as Map<String, dynamic>),
        bootstrapped: true,
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        await storage.clear();
      }
      state = state.copyWith(bootstrapped: true);
    } catch (_) {
      state = state.copyWith(bootstrapped: true);
    }
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
      state = AuthState(
        user: AppUser.fromJson(data['user'] as Map<String, dynamic>),
        bootstrapped: true,
      );
    } catch (e) {
      if (kDebugMode) {
        debugPrint('apple sign in failed: $e');
      }
      state = state.copyWith(loading: false, error: '登录失败，请重试');
    }
  }

  Future<void> signOut() async {
    await _ref.read(tokenStorageProvider).clear();
    state = AuthState(bootstrapped: true);
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(ref);
});

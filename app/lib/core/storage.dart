import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

class TokenStorage {
  TokenStorage(this._prefs);
  final SharedPreferences _prefs;
  static const _key = 'agentzero.token';

  String? read() => _prefs.getString(_key);
  Future<void> save(String token) => _prefs.setString(_key, token);
  Future<void> clear() => _prefs.remove(_key);
}

/// 同步 SharedPreferences provider —— 必须由 main() 在 `await SharedPreferences.getInstance()`
/// 后通过 `overrideWithValue(...)` 注入，否则首次 read 时会立即抛错。
///
/// 这样设计是为了避免 AuthNotifier / ApiClient 在构造时拿不到 prefs（FutureProvider
/// 第一次 read 时还在 loading → requireValue 抛错 → 静默吞掉 → 永远未登录）。
final sharedPreferencesProvider = Provider<SharedPreferences>((ref) {
  throw UnimplementedError(
    'sharedPreferencesProvider 必须在 main() 用 override 注入',
  );
});

final tokenStorageProvider = Provider<TokenStorage>((ref) {
  return TokenStorage(ref.watch(sharedPreferencesProvider));
});

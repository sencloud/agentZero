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

final sharedPreferencesProvider = FutureProvider<SharedPreferences>((ref) async {
  return SharedPreferences.getInstance();
});

final tokenStorageProvider = Provider<TokenStorage>((ref) {
  final prefs = ref.watch(sharedPreferencesProvider).requireValue;
  return TokenStorage(prefs);
});

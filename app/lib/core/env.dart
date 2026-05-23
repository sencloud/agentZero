class AppEnv {
  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'https://47.96.115.180.nip.io',
  );

  static const String appleBundleId = String.fromEnvironment(
    'APPLE_BUNDLE_ID',
    defaultValue: 'com.agentzero.me',
  );
}

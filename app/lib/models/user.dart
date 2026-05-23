class AppUser {
  AppUser({required this.id, required this.nickname, required this.email, required this.avatarUrl});
  final int id;
  final String nickname;
  final String email;
  final String avatarUrl;

  factory AppUser.fromJson(Map<String, dynamic> json) => AppUser(
        id: (json['id'] as num).toInt(),
        nickname: (json['nickname'] as String?) ?? 'AgentZero 用户',
        email: (json['email'] as String?) ?? '',
        avatarUrl: (json['avatar_url'] as String?) ?? '',
      );
}

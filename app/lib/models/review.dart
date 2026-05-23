class Review {
  Review({
    required this.id,
    required this.nickname,
    required this.avatar,
    required this.rating,
    required this.title,
    required this.body,
    required this.createdAt,
  });

  final int id;
  final String nickname;
  final String avatar;
  final int rating;
  final String title;
  final String body;
  final DateTime createdAt;

  factory Review.fromJson(Map<String, dynamic> json) => Review(
        id: (json['id'] as num).toInt(),
        nickname: (json['nickname'] as String?) ?? '匿名用户',
        avatar: (json['avatar'] as String?) ?? '',
        rating: (json['rating'] as num).toInt(),
        title: (json['title'] as String?) ?? '',
        body: (json['body'] as String?) ?? '',
        createdAt: DateTime.parse(json['created_at'] as String),
      );
}

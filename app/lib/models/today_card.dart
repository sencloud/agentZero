class TodayCard {
  TodayCard({
    required this.id,
    required this.kind,
    required this.eyebrow,
    required this.title,
    required this.subtitle,
    required this.coverUrl,
    required this.agentSlug,
  });

  final int id;
  final String kind;
  final String eyebrow;
  final String title;
  final String subtitle;
  final String coverUrl;
  final String agentSlug;

  factory TodayCard.fromJson(Map<String, dynamic> json) => TodayCard(
        id: (json['id'] as num).toInt(),
        kind: json['kind'] as String,
        eyebrow: json['eyebrow'] as String,
        title: json['title'] as String,
        subtitle: json['subtitle'] as String,
        coverUrl: json['cover_url'] as String,
        agentSlug: (json['agent_slug'] as String?) ?? '',
      );
}

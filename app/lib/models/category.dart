class AgentCategory {
  AgentCategory({
    required this.id,
    required this.slug,
    required this.name,
    required this.icon,
    required this.color,
  });

  final int id;
  final String slug;
  final String name;
  final String icon;
  final String color;

  factory AgentCategory.fromJson(Map<String, dynamic> json) => AgentCategory(
        id: (json['id'] as num).toInt(),
        slug: json['slug'] as String,
        name: json['name'] as String,
        icon: json['icon'] as String,
        color: json['color'] as String,
      );
}

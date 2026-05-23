class Agent {
  Agent({
    required this.id,
    required this.slug,
    required this.name,
    required this.tagline,
    required this.description,
    required this.iconUrl,
    required this.coverUrl,
    required this.screenshots,
    required this.categoryName,
    required this.categorySlug,
    required this.developer,
    required this.version,
    required this.sizeBytes,
    required this.rating,
    required this.ratingCount,
    required this.installCount,
    required this.isFree,
    required this.priceCents,
    required this.isFeatured,
    required this.featureBadge,
    required this.capabilities,
    required this.updatedNotes,
    required this.installed,
  });

  final int id;
  final String slug;
  final String name;
  final String tagline;
  final String description;
  final String iconUrl;
  final String coverUrl;
  final List<String> screenshots;
  final String categoryName;
  final String categorySlug;
  final String developer;
  final String version;
  final int sizeBytes;
  final double rating;
  final int ratingCount;
  final int installCount;
  final bool isFree;
  final int priceCents;
  final bool isFeatured;
  final String featureBadge;
  final List<String> capabilities;
  final String updatedNotes;
  final bool installed;

  factory Agent.fromJson(Map<String, dynamic> json) {
    return Agent(
      id: (json['id'] as num).toInt(),
      slug: json['slug'] as String,
      name: json['name'] as String,
      tagline: json['tagline'] as String,
      description: json['description'] as String,
      iconUrl: json['icon_url'] as String,
      coverUrl: json['cover_url'] as String,
      screenshots: (json['screenshots'] as List? ?? const []).map((e) => e as String).toList(),
      categoryName: (json['category_name'] as String?) ?? '',
      categorySlug: (json['category_slug'] as String?) ?? '',
      developer: json['developer'] as String,
      version: json['version'] as String,
      sizeBytes: (json['size_bytes'] as num).toInt(),
      rating: ((json['rating'] as num?) ?? 0).toDouble(),
      ratingCount: (json['rating_count'] as num?)?.toInt() ?? 0,
      installCount: (json['install_count'] as num?)?.toInt() ?? 0,
      isFree: json['is_free'] as bool? ?? true,
      priceCents: (json['price_cents'] as num?)?.toInt() ?? 0,
      isFeatured: json['is_featured'] as bool? ?? false,
      featureBadge: (json['feature_badge'] as String?) ?? '',
      capabilities: (json['capabilities'] as List? ?? const []).map((e) => e as String).toList(),
      updatedNotes: (json['updated_notes'] as String?) ?? '',
      installed: json['installed'] as bool? ?? false,
    );
  }

  Agent copyWith({bool? installed}) => Agent(
        id: id,
        slug: slug,
        name: name,
        tagline: tagline,
        description: description,
        iconUrl: iconUrl,
        coverUrl: coverUrl,
        screenshots: screenshots,
        categoryName: categoryName,
        categorySlug: categorySlug,
        developer: developer,
        version: version,
        sizeBytes: sizeBytes,
        rating: rating,
        ratingCount: ratingCount,
        installCount: installCount,
        isFree: isFree,
        priceCents: priceCents,
        isFeatured: isFeatured,
        featureBadge: featureBadge,
        capabilities: capabilities,
        updatedNotes: updatedNotes,
        installed: installed ?? this.installed,
      );
}

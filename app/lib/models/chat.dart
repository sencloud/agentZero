class ChatSession {
  ChatSession({required this.id, required this.agentId, required this.title, required this.createdAt});
  final int id;
  final int agentId;
  final String title;
  final DateTime createdAt;

  factory ChatSession.fromJson(Map<String, dynamic> json) => ChatSession(
        id: (json['id'] as num).toInt(),
        agentId: (json['agent_id'] as num).toInt(),
        title: (json['title'] as String?) ?? '',
        createdAt: DateTime.parse(json['created_at'] as String),
      );
}

class ChatMessage {
  ChatMessage({required this.id, required this.role, required this.content, required this.createdAt});
  final int id;
  final String role;
  final String content;
  final DateTime createdAt;

  bool get isUser => role == 'user';

  factory ChatMessage.fromJson(Map<String, dynamic> json) => ChatMessage(
        id: (json['id'] as num).toInt(),
        role: json['role'] as String,
        content: json['content'] as String,
        createdAt: DateTime.parse(json['created_at'] as String),
      );
}

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../models/chat.dart';

class ChatState {
  ChatState({this.session, this.messages = const [], this.sending = false, this.error});
  final ChatSession? session;
  final List<ChatMessage> messages;
  final bool sending;
  final String? error;

  ChatState copyWith({ChatSession? session, List<ChatMessage>? messages, bool? sending, String? error, bool clearError = false}) =>
      ChatState(
        session: session ?? this.session,
        messages: messages ?? this.messages,
        sending: sending ?? this.sending,
        error: clearError ? null : (error ?? this.error),
      );
}

class ChatNotifier extends StateNotifier<ChatState> {
  ChatNotifier(this._ref, this._agentSlug) : super(ChatState()) {
    _open();
  }

  final Ref _ref;
  final String _agentSlug;

  Future<void> _open() async {
    final api = _ref.read(apiClientProvider);
    final r = await api.dio.post('/agents/$_agentSlug/sessions');
    final data = r.data as Map<String, dynamic>;
    final session = ChatSession.fromJson(data['session'] as Map<String, dynamic>);
    final greeting = data['greeting'] != null
        ? [ChatMessage.fromJson(data['greeting'] as Map<String, dynamic>)]
        : <ChatMessage>[];
    state = state.copyWith(session: session, messages: greeting);
  }

  Future<void> send(String text) async {
    final session = state.session;
    if (session == null) return;
    state = state.copyWith(sending: true, clearError: true);
    try {
      final api = _ref.read(apiClientProvider);
      final r = await api.dio.post('/sessions/${session.id}/messages', data: {'content': text});
      final data = r.data as Map<String, dynamic>;
      final user = ChatMessage.fromJson(data['user'] as Map<String, dynamic>);
      final assistant = ChatMessage.fromJson(data['assistant'] as Map<String, dynamic>);
      state = state.copyWith(messages: [...state.messages, user, assistant], sending: false);
    } catch (e) {
      state = state.copyWith(sending: false, error: '消息发送失败');
    }
  }
}

final chatProvider = StateNotifierProvider.autoDispose.family<ChatNotifier, ChatState, String>((ref, slug) {
  return ChatNotifier(ref, slug);
});

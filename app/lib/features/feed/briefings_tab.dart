import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_html/flutter_html.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/api_client.dart';
import '../../core/theme.dart';
import '../../models/feed.dart';
import '../../providers/feed.dart';

/// 简报 tab：列表 + 顶部最新简报头卡 + 「立即生成」按钮 + 进度面板。
class BriefingsTab extends ConsumerWidget {
  const BriefingsTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final list = ref.watch(briefingsListProvider);
    final gen = ref.watch(briefingGenProvider);

    return Column(
      children: [
        _TopBar(generating: gen.running),
        if (gen.events.isNotEmpty || gen.running) _GenProgressPanel(state: gen),
        Expanded(
          child: list.when(
            loading: () => const Center(
              child: SizedBox(
                width: 22,
                height: 22,
                child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
              ),
            ),
            error: (e, _) => Center(
              child: Text('加载简报失败：$e',
                  style: const TextStyle(color: AppTheme.redline, fontSize: 12)),
            ),
            data: (rows) {
              if (rows.isEmpty) {
                return const _EmptyBriefings();
              }
              return RefreshIndicator(
                onRefresh: () async => ref.invalidate(briefingsListProvider),
                color: AppTheme.paper,
                backgroundColor: AppTheme.carbon,
                child: ListView.separated(
                  padding: const EdgeInsets.fromLTRB(12, 12, 12, 60),
                  itemCount: rows.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 10),
                  itemBuilder: (_, i) => _BriefingCard(briefing: rows[i], featured: i == 0),
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}

class _TopBar extends ConsumerWidget {
  const _TopBar({required this.generating});
  final bool generating;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 10, 12, 10),
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: AppTheme.graphite, width: 0.6)),
      ),
      child: Row(
        children: [
          const Icon(CupertinoIcons.doc_text_search, color: AppTheme.amber, size: 16),
          const SizedBox(width: 8),
          const Text('AI 情报简报',
              style: TextStyle(
                color: AppTheme.paper,
                fontSize: 13,
                letterSpacing: 4,
                fontWeight: FontWeight.w700,
              )),
          const Spacer(),
          OutlinedButton.icon(
            onPressed: generating
                ? null
                : () => ref.read(briefingGenProvider.notifier).start(window: '1h'),
            style: OutlinedButton.styleFrom(
              foregroundColor: AppTheme.paper,
              side: const BorderSide(color: AppTheme.redline),
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            ),
            icon: generating
                ? const SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTheme.paper))
                : const Icon(CupertinoIcons.sparkles, size: 14, color: AppTheme.redline),
            label: const Text('立即生成',
                style: TextStyle(fontSize: 11, letterSpacing: 2)),
          ),
        ],
      ),
    );
  }
}

class _EmptyBriefings extends StatelessWidget {
  const _EmptyBriefings();

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(horizontal: 28, vertical: 60),
      child: Column(
        children: [
          Icon(CupertinoIcons.doc_text, size: 36, color: AppTheme.muted),
          SizedBox(height: 14),
          Text('尚无简报',
              style: TextStyle(
                  color: AppTheme.paper,
                  fontSize: 13,
                  letterSpacing: 4,
                  fontWeight: FontWeight.w600)),
          SizedBox(height: 8),
          Text(
            '请先在「图谱」tab 添加你关心的话题。\n后台每小时会自动用 DeepSeek 跑一份情报简报，\n你也可以点右上「立即生成」马上得到一份。',
            textAlign: TextAlign.center,
            style: TextStyle(color: AppTheme.muted, fontSize: 12, height: 1.6),
          ),
        ],
      ),
    );
  }
}

class _BriefingCard extends StatelessWidget {
  const _BriefingCard({required this.briefing, this.featured = false});
  final Briefing briefing;
  final bool featured;

  @override
  Widget build(BuildContext context) {
    final t = briefing.generatedAt.toLocal();
    final timeStr = DateFormat('MM-dd HH:mm').format(t);
    return InkWell(
      onTap: () => Navigator.of(context).push(MaterialPageRoute(
        builder: (_) => BriefingDetailPage(briefing: briefing),
      )),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: featured ? AppTheme.carbon : AppTheme.ink,
          border: Border.all(
            color: featured ? AppTheme.redline.withValues(alpha: 0.7) : AppTheme.graphite,
            width: featured ? 1.2 : 0.6,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _WindowChip(window: briefing.window),
                const SizedBox(width: 8),
                Text(timeStr,
                    style: const TextStyle(
                      color: AppTheme.muted,
                      fontSize: 10,
                      letterSpacing: 2,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
                const Spacer(),
                if (featured)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                    decoration: BoxDecoration(border: Border.all(color: AppTheme.amber, width: 0.8)),
                    child: const Text('LATEST',
                        style: TextStyle(
                          color: AppTheme.amber,
                          fontSize: 9,
                          letterSpacing: 2,
                          fontFamilyFallback: AppTheme.monoFallback,
                        )),
                  ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              briefing.title.isEmpty ? '(无标题)' : briefing.title,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                color: AppTheme.paper,
                fontSize: featured ? 18 : 15,
                fontWeight: FontWeight.w700,
                height: 1.4,
                letterSpacing: 1,
              ),
            ),
            if (briefing.summary.isNotEmpty) ...[
              const SizedBox(height: 6),
              Text(
                briefing.summary,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: AppTheme.pen, fontSize: 12.5, height: 1.5),
              ),
            ],
            const SizedBox(height: 10),
            Row(
              children: [
                Text('${briefing.eventCount} 事件',
                    style: const TextStyle(
                      color: AppTheme.muted,
                      fontSize: 10,
                      letterSpacing: 1.5,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
                const SizedBox(width: 10),
                Text('${briefing.clusterCount} 主题簇',
                    style: const TextStyle(
                      color: AppTheme.muted,
                      fontSize: 10,
                      letterSpacing: 1.5,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
                const Spacer(),
                const Icon(CupertinoIcons.chevron_right, size: 12, color: AppTheme.muted),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _WindowChip extends StatelessWidget {
  const _WindowChip({required this.window});
  final String window;

  @override
  Widget build(BuildContext context) {
    final label = switch (window) { '1h' => '过去 1 小时', '24h' => '过去 24 小时', '7d' => '过去 7 天', _ => window };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(border: Border.all(color: AppTheme.amber, width: 0.6)),
      child: Text(label,
          style: const TextStyle(
            color: AppTheme.amber,
            fontSize: 9.5,
            letterSpacing: 2,
            fontFamilyFallback: AppTheme.monoFallback,
          )),
    );
  }
}

// ---------------------------------------------------------------------------
// 生成进度面板（与 Refresh 类似，但走 /briefings/generate/stream）
// ---------------------------------------------------------------------------

class BriefingGenEvent {
  BriefingGenEvent(this.phase, this.message);
  final String phase;
  final String message;
}

class BriefingGenState {
  const BriefingGenState({required this.running, required this.events, this.error});
  final bool running;
  final List<BriefingGenEvent> events;
  final String? error;

  static const idle = BriefingGenState(running: false, events: []);
}

class BriefingGenNotifier extends StateNotifier<BriefingGenState> {
  BriefingGenNotifier(this._ref) : super(BriefingGenState.idle);
  final Ref _ref;
  StreamSubscription? _sub;

  Future<void> start({String window = '1h'}) async {
    if (state.running) return;
    state = const BriefingGenState(running: true, events: []);
    final dio = _ref.read(apiClientProvider).dio;
    try {
      final res = await dio.get<ResponseBody>(
        '/briefings/generate/stream',
        queryParameters: {'window': window},
        options: Options(
          responseType: ResponseType.stream,
          headers: {'Accept': 'text/event-stream'},
          receiveTimeout: Duration.zero,
        ),
      );
      final body = res.data;
      if (body == null) {
        state = BriefingGenState(running: false, events: state.events, error: '空响应');
        return;
      }
      final buffer = StringBuffer();
      final completer = Completer<void>();
      _sub = body.stream.cast<List<int>>().listen(
        (chunk) {
          buffer.write(utf8.decode(chunk, allowMalformed: true));
          final parts = buffer.toString().split('\n\n');
          for (var i = 0; i < parts.length - 1; i++) {
            final raw = parts[i].trim();
            if (!raw.startsWith('data:')) continue;
            try {
              final j = Map<String, dynamic>.from(jsonDecode(raw.substring(5).trim()));
              final ev = BriefingGenEvent(
                (j['phase'] as String?) ?? 'unknown',
                (j['message'] as String?) ?? '',
              );
              state = BriefingGenState(running: true, events: [...state.events, ev]);
              if (ev.phase == 'done' || ev.phase == 'error') {
                _ref.invalidate(briefingsListProvider);
              }
            } catch (_) {}
          }
          buffer
            ..clear()
            ..write(parts.last);
        },
        onDone: () {
          state = BriefingGenState(running: false, events: state.events);
          if (!completer.isCompleted) completer.complete();
        },
        onError: (e) {
          state = BriefingGenState(running: false, events: state.events, error: e.toString());
          if (!completer.isCompleted) completer.complete();
        },
        cancelOnError: true,
      );
      await completer.future;
    } catch (e) {
      state = BriefingGenState(running: false, events: state.events, error: e.toString());
    }
  }

  void clear() => state = BriefingGenState.idle;

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }
}

final briefingGenProvider =
    StateNotifierProvider<BriefingGenNotifier, BriefingGenState>((ref) => BriefingGenNotifier(ref));

class _GenProgressPanel extends ConsumerWidget {
  const _GenProgressPanel({required this.state});
  final BriefingGenState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 10, 12, 0),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: AppTheme.carbon,
        border: Border.all(color: AppTheme.graphite, width: 0.6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              if (state.running)
                const SizedBox(
                  width: 12,
                  height: 12,
                  child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTheme.paper),
                )
              else
                Icon(state.error != null ? CupertinoIcons.exclamationmark : CupertinoIcons.check_mark,
                    size: 14,
                    color: state.error != null ? AppTheme.redline : AppTheme.amber),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  state.running ? '简报生成中…' : (state.error ?? '本次生成已完成'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: state.error != null ? AppTheme.redline : AppTheme.paper,
                    fontSize: 11,
                    letterSpacing: 2,
                  ),
                ),
              ),
              if (!state.running)
                IconButton(
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(minWidth: 24, minHeight: 24),
                  icon: const Icon(CupertinoIcons.xmark, size: 12, color: AppTheme.muted),
                  onPressed: () => ref.read(briefingGenProvider.notifier).clear(),
                ),
            ],
          ),
          if (state.events.isNotEmpty) ...[
            const SizedBox(height: 8),
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 140),
              child: ListView.builder(
                shrinkWrap: true,
                itemCount: state.events.length,
                itemBuilder: (_, i) {
                  final e = state.events[i];
                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 2),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
                          decoration: BoxDecoration(
                              border: Border.all(color: _phaseColor(e.phase), width: 0.6)),
                          child: Text(_phaseTag(e.phase),
                              style: TextStyle(
                                color: _phaseColor(e.phase),
                                fontSize: 9,
                                letterSpacing: 1.5,
                                fontFamilyFallback: AppTheme.monoFallback,
                              )),
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(e.message,
                              style: const TextStyle(
                                color: AppTheme.paper,
                                fontSize: 11,
                                height: 1.5,
                              )),
                        ),
                      ],
                    ),
                  );
                },
              ),
            ),
          ],
        ],
      ),
    );
  }

  static String _phaseTag(String p) => switch (p) {
        'start' => 'START',
        'gather' => 'GATHER',
        'cluster' || 'cluster_done' => 'CLUSTER',
        'correlate' || 'correlate_done' => 'CORREL',
        'write' || 'write_done' => 'WRITE',
        'save' => 'SAVE',
        'done' => 'DONE',
        'error' => 'ERR',
        _ => p.toUpperCase(),
      };

  static Color _phaseColor(String p) => switch (p) {
        'cluster_done' || 'correlate_done' || 'write_done' || 'save' || 'done' => AppTheme.amber,
        'error' => AppTheme.redline,
        _ => AppTheme.pen,
      };
}

// ---------------------------------------------------------------------------
// 详情页：HTML 富媒体渲染
// ---------------------------------------------------------------------------

class BriefingDetailPage extends ConsumerWidget {
  const BriefingDetailPage({super.key, required this.briefing});
  final Briefing briefing;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final html = ref.watch(briefingHtmlProvider(briefing.id));
    return Scaffold(
      appBar: AppBar(
        title: Text('简报 · ${briefing.window}',
            style: const TextStyle(color: AppTheme.paper, fontSize: 13, letterSpacing: 3)),
      ),
      body: SafeArea(
        top: false,
        child: html.when(
          loading: () => const Center(
              child: SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper))),
          error: (e, _) => Center(
              child: Text('正文加载失败：$e',
                  style: const TextStyle(color: AppTheme.redline, fontSize: 12))),
          data: (raw) => SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
            child: Html(data: raw, style: _htmlStyle),
          ),
        ),
      ),
    );
  }
}

// flutter_html 的样式映射，复用机密文档风格
final Map<String, Style> _htmlStyle = {
  'body': Style(
    backgroundColor: AppTheme.ink,
    color: AppTheme.paper,
    fontSize: FontSize(14),
    margin: Margins.zero,
    padding: HtmlPaddings.zero,
  ),
  'h1': Style(
    color: AppTheme.paper,
    fontSize: FontSize(20),
    fontWeight: FontWeight.w700,
    letterSpacing: 2,
  ),
  'h3': Style(
    color: AppTheme.amber,
    fontSize: FontSize(13),
    letterSpacing: 4,
    margin: Margins.only(top: 22, bottom: 8),
  ),
  'h4': Style(
    color: AppTheme.paper,
    fontSize: FontSize(15),
    fontWeight: FontWeight.w600,
    margin: Margins.only(top: 10, bottom: 2),
  ),
  'p': Style(color: AppTheme.pen, lineHeight: LineHeight(1.7)),
  'li': Style(color: AppTheme.pen, lineHeight: LineHeight(1.7)),
  'b': Style(color: AppTheme.paper, fontWeight: FontWeight.w700),
  '.summary': Style(
    backgroundColor: AppTheme.carbon,
    color: AppTheme.paper,
    padding: HtmlPaddings.symmetric(horizontal: 14, vertical: 12),
    border: const Border(left: BorderSide(color: AppTheme.redline, width: 3)),
    margin: Margins.symmetric(vertical: 10),
  ),
  '.meta': Style(color: AppTheme.muted, fontSize: FontSize(11), letterSpacing: 1),
  '.dir.up': Style(color: const Color(0xFF67D17B)),
  '.dir.down': Style(color: AppTheme.redline),
  '.dir.watch': Style(color: AppTheme.amber),
};

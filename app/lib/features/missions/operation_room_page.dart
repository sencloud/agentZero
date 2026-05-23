import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../core/api_client.dart';
import '../../core/theme.dart';
import '../../models/mission.dart';
import '../../providers/missions.dart';
import 'sse_client.dart';

/// 行动现场（M3d）。
///
/// 三个分区：
///   1. 顶部头条 - 代号、状态、token 计费、撤离按钮
///   2. 事件流 - 把 steps 折叠 + 渲染成电报式条目
///   3. 底部抽屉 - 入柜工件列表
class OperationRoomPage extends ConsumerStatefulWidget {
  const OperationRoomPage({super.key, required this.missionId});
  final String missionId;

  @override
  ConsumerState<OperationRoomPage> createState() => _OperationRoomPageState();
}

class _OperationRoomPageState extends ConsumerState<OperationRoomPage> {
  Mission? _mission;
  List<MissionStep> _steps = [];
  List<Artifact> _artifacts = [];
  bool _running = false;
  bool _loading = true;
  String? _error;

  StreamSubscription<MissionStep>? _subscription;
  final ScrollController _scrollCtrl = ScrollController();
  bool _autoScroll = true;

  /// 任务的"最终汇报"工件（按命名约定优先取 报告.html / report.html，再次取
  /// 最后一个 HTML 工件，再次为空）。
  Artifact? get _reportArtifact {
    if (_artifacts.isEmpty) return null;
    Artifact? named;
    Artifact? lastHtml;
    for (final a in _artifacts) {
      final isHtml = a.mime.startsWith('text/html') ||
          a.name.toLowerCase().endsWith('.html') ||
          a.name.toLowerCase().endsWith('.htm');
      if (!isHtml) continue;
      if (a.name == '报告.html' || a.name.toLowerCase() == 'report.html') {
        named = a;
      }
      lastHtml = a;
    }
    return named ?? lastHtml;
  }

  @override
  void initState() {
    super.initState();
    _scrollCtrl.addListener(_onUserScroll);
    _bootstrap();
  }

  @override
  void dispose() {
    _subscription?.cancel();
    _scrollCtrl
      ..removeListener(_onUserScroll)
      ..dispose();
    super.dispose();
  }

  // 用户主动往上滑就别强制吸底了
  void _onUserScroll() {
    if (!_scrollCtrl.hasClients) return;
    final atBottom = _scrollCtrl.position.pixels >= _scrollCtrl.position.maxScrollExtent - 80;
    if (_autoScroll != atBottom) {
      setState(() => _autoScroll = atBottom);
    }
  }

  Future<void> _bootstrap() async {
    try {
      final detail = await ref.read(missionDetailProvider(widget.missionId).future);
      if (!mounted) return;
      setState(() {
        _mission = detail.mission;
        _steps = detail.steps;
        _artifacts = detail.artifacts;
        _running = detail.running;
        _loading = false;
      });
      _scrollToBottomSoon();
      if (!_mission!.status.isTerminal) _subscribe();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '$e';
      });
    }
  }

  void _subscribe() {
    final api = ref.read(apiClientProvider);
    final sse = MissionEventStream(api.dio);
    final afterSeq = _steps.isEmpty ? 0 : _steps.last.seq;
    _subscription = sse.connect(widget.missionId, afterSeq: afterSeq).listen(
      (step) {
        if (!mounted) return;
        setState(() {
          _steps = [..._steps, step];
          // 同时把工件事件折射到 artifacts 列表（避免重复一次详情请求）
          if (step.type == 'artifact') {
            _artifacts = [
              Artifact(
                id: (step.payload['artifact_id'] as num?)?.toInt() ?? 0,
                missionId: step.missionId,
                kind: (step.payload['kind'] as String?) ?? 'file',
                name: (step.payload['name'] as String?) ?? '',
                path: (step.payload['path'] as String?) ?? '',
                mime: (step.payload['mime'] as String?) ?? '',
                size: (step.payload['size'] as num?)?.toInt() ?? 0,
                createdAt: step.ts,
              ),
              ..._artifacts,
            ];
          }
          // task 终态时立刻反映到顶部头条
          if (step.type == 'system') {
            final kind = step.payload['kind'] as String?;
            if (kind == 'task_done') {
              _mission = _mission?.copyWithStatus(MissionStatus.done);
              _running = false;
            } else if (kind == 'error' || kind == 'max_iter') {
              _mission = _mission?.copyWithStatus(MissionStatus.error);
              _running = false;
            }
          }
        });
        _scrollToBottomSoon();
      },
      onDone: () {
        if (!mounted) return;
        setState(() => _running = false);
        // 刷新 mission 状态（broker 关闭意味着进入终态）
        ref.invalidate(missionDetailProvider(widget.missionId));
      },
      onError: (e) {
        if (!mounted) return;
        setState(() => _error = '事件流断开：$e');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            backgroundColor: AppTheme.redline,
            content: Text('事件流断开：$e',
                style: const TextStyle(color: AppTheme.paper)),
            duration: const Duration(seconds: 5),
          ),
        );
      },
    );
  }

  void _scrollToBottomSoon() {
    if (!_autoScroll) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollCtrl.hasClients) return;
      _scrollCtrl.jumpTo(_scrollCtrl.position.maxScrollExtent);
    });
  }

  Future<void> _confirmAbort() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => _AbortDialog(),
    );
    if (ok != true) return;
    try {
      await ref.read(abortMissionProvider).call(widget.missionId);
    } catch (_) {}
  }

  void _openVault() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: AppTheme.carbon,
      showDragHandle: false,
      isScrollControlled: true,
      builder: (ctx) => _VaultSheet(artifacts: _artifacts),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator(color: AppTheme.paper)));
    }
    if (_error != null && _mission == null) {
      return Scaffold(
        appBar: AppBar(),
        body: Center(child: Text(_error!, style: const TextStyle(color: AppTheme.redline))),
      );
    }
    final m = _mission!;
    final blocks = _foldSteps(_steps);

    return Scaffold(
      appBar: AppBar(
        title: Row(
          children: [
            AppDecor.stamp(
              m.status.label,
              border: _statusColor(m.status),
              color: _statusColor(m.status),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                m.codename,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  color: AppTheme.paper,
                  fontSize: 16,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 3,
                ),
              ),
            ),
          ],
        ),
        actions: [
          if (_running)
            IconButton(
              tooltip: '撤离',
              icon: const Icon(CupertinoIcons.power, color: AppTheme.redline),
              onPressed: _confirmAbort,
            ),
          IconButton(
            tooltip: '工件柜',
            icon: const Icon(CupertinoIcons.tray_arrow_down, color: AppTheme.paper),
            onPressed: _openVault,
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _MissionHeader(mission: m, artifactCount: _artifacts.length),
            const Divider(height: 1, color: AppTheme.graphite),
            if (_reportArtifact != null) _ReportBanner(mission: m, artifact: _reportArtifact!),
            Expanded(
              child: blocks.isEmpty
                  ? _EmptyEventState(running: _running)
                  : Stack(
                      children: [
                        ListView.builder(
                          controller: _scrollCtrl,
                          padding: const EdgeInsets.fromLTRB(16, 16, 16, 120),
                          itemCount: blocks.length,
                          itemBuilder: (_, i) => _EventBlockView(block: blocks[i]),
                        ),
                        if (!_autoScroll)
                          Positioned(
                            right: 16,
                            bottom: 16,
                            child: _ScrollDownChip(onTap: () {
                              setState(() => _autoScroll = true);
                              _scrollToBottomSoon();
                            }),
                          ),
                      ],
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

// ============== 作战汇报横幅（任务出 HTML 工件后顶部出现） ==============

class _ReportBanner extends StatelessWidget {
  const _ReportBanner({required this.mission, required this.artifact});
  final Mission mission;
  final Artifact artifact;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: () {
          context.push(
            '/missions/${mission.id}/artifacts/${artifact.id}'
            '?name=${Uri.encodeQueryComponent(artifact.name)}'
            '&mime=${Uri.encodeQueryComponent(artifact.mime)}',
          );
        },
        child: Container(
          margin: const EdgeInsets.fromLTRB(16, 12, 16, 4),
          padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
          decoration: BoxDecoration(
            color: AppTheme.paper,
            border: Border.all(color: AppTheme.redline, width: 1.2),
          ),
          child: Row(
            children: [
              Container(
                width: 36,
                height: 36,
                alignment: Alignment.center,
                decoration: BoxDecoration(border: Border.all(color: AppTheme.redline, width: 1)),
                child: const Icon(CupertinoIcons.doc_richtext, color: AppTheme.redline, size: 18),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      '作战汇报已就绪',
                      style: TextStyle(
                        color: AppTheme.ink,
                        fontSize: 14,
                        fontWeight: FontWeight.w800,
                        letterSpacing: 3,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      artifact.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: AppTheme.muted,
                        fontSize: 11,
                        letterSpacing: 1,
                        fontFamilyFallback: AppTheme.monoFallback,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              const Icon(CupertinoIcons.arrow_right_circle_fill, color: AppTheme.redline, size: 22),
            ],
          ),
        ),
      ),
    );
  }
}

// ============== 头部状态条 ==============

class _MissionHeader extends StatelessWidget {
  const _MissionHeader({required this.mission, required this.artifactCount});
  final Mission mission;
  final int artifactCount;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 14),
      color: AppTheme.ink,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(mission.brief,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: AppTheme.pen, fontSize: 13, height: 1.5)),
          const SizedBox(height: 10),
          Row(
            children: [
              _MetaInline(label: 'TIER', value: mission.tier.label),
              const SizedBox(width: 14),
              _MetaInline(label: 'KIT', value: mission.loadout.length.toString()),
              const SizedBox(width: 14),
              _MetaInline(label: 'IN', value: '${mission.inputTokens}t'),
              const SizedBox(width: 14),
              _MetaInline(label: 'OUT', value: '${mission.outputTokens}t'),
              const Spacer(),
              _MetaInline(label: 'VAULT', value: artifactCount.toString()),
            ],
          ),
        ],
      ),
    );
  }
}

class _MetaInline extends StatelessWidget {
  const _MetaInline({required this.label, required this.value});
  final String label;
  final String value;
  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Text(label,
            style: const TextStyle(
              color: AppTheme.muted,
              fontSize: 9,
              letterSpacing: 2,
              fontWeight: FontWeight.w700,
              fontFamilyFallback: AppTheme.monoFallback,
            )),
        const SizedBox(width: 4),
        Text(value,
            style: const TextStyle(
              color: AppTheme.paper,
              fontSize: 11,
              letterSpacing: 1,
              fontWeight: FontWeight.w600,
              fontFamilyFallback: AppTheme.monoFallback,
            )),
      ],
    );
  }
}

Color _statusColor(MissionStatus s) {
  switch (s) {
    case MissionStatus.running:
      return AppTheme.redline;
    case MissionStatus.done:
      return AppTheme.sage;
    case MissionStatus.aborted:
    case MissionStatus.error:
      return AppTheme.amber;
    case MissionStatus.pending:
      return AppTheme.pen;
  }
}

// ============== 事件流：折叠 + 渲染 ==============

/// 一个语义块，可能由若干条同类 step 拼接而成。
class _Block {
  _Block({required this.kind, required this.ts, required this.text, this.payload});
  final String kind; // thought / message / tool_call / tool_result / artifact / system
  final DateTime ts;
  final String text;
  final Map<String, dynamic>? payload;
}

/// 把相邻的 thought / message delta 合并成一段，方便阅读。
/// 其他类型一律保留为单独 block。
List<_Block> _foldSteps(List<MissionStep> steps) {
  final out = <_Block>[];
  StringBuffer? buf;
  String? bufKind;
  DateTime? bufTs;

  void flushBuf() {
    if (buf != null && buf!.isNotEmpty) {
      out.add(_Block(kind: bufKind!, ts: bufTs!, text: buf!.toString()));
    }
    buf = null;
    bufKind = null;
    bufTs = null;
  }

  for (final s in steps) {
    if (s.type == 'thought' || s.type == 'message') {
      if (bufKind == s.type) {
        buf!.write((s.payload['text'] as String?) ?? '');
      } else {
        flushBuf();
        bufKind = s.type;
        bufTs = s.ts;
        buf = StringBuffer((s.payload['text'] as String?) ?? '');
      }
      continue;
    }
    flushBuf();
    out.add(_Block(kind: s.type, ts: s.ts, text: '', payload: s.payload));
  }
  flushBuf();
  return out;
}

class _EventBlockView extends StatelessWidget {
  const _EventBlockView({required this.block});
  final _Block block;

  @override
  Widget build(BuildContext context) {
    switch (block.kind) {
      case 'thought':
        return _ThoughtBlock(text: block.text, ts: block.ts);
      case 'message':
        return _MessageBlock(text: block.text, ts: block.ts);
      case 'tool_call':
        return _ToolCallBlock(payload: block.payload!, ts: block.ts);
      case 'tool_result':
        return _ToolResultBlock(payload: block.payload!, ts: block.ts);
      case 'artifact':
        return _ArtifactBlock(payload: block.payload!, ts: block.ts);
      case 'system':
        return _SystemBlock(payload: block.payload!, ts: block.ts);
      case 'usage':
        return const SizedBox.shrink(); // 不直接显示，已聚合到头部
      default:
        return _SystemBlock(payload: {'text': block.text, 'kind': block.kind}, ts: block.ts);
    }
  }
}

String _hhmmss(DateTime t) => DateFormat('HH:mm:ss').format(t);

class _Timestamp extends StatelessWidget {
  const _Timestamp({required this.ts});
  final DateTime ts;
  @override
  Widget build(BuildContext context) {
    return Text(
      _hhmmss(ts),
      style: const TextStyle(
        color: AppTheme.muted,
        fontSize: 10,
        letterSpacing: 2,
        fontFamilyFallback: AppTheme.monoFallback,
      ),
    );
  }
}

class _ThoughtBlock extends StatefulWidget {
  const _ThoughtBlock({required this.text, required this.ts});
  final String text;
  final DateTime ts;

  @override
  State<_ThoughtBlock> createState() => _ThoughtBlockState();
}

class _ThoughtBlockState extends State<_ThoughtBlock> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final chars = widget.text.length;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => setState(() => _expanded = !_expanded),
          child: AnimatedSize(
            duration: const Duration(milliseconds: 160),
            alignment: Alignment.topLeft,
            curve: Curves.easeOutCubic,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(width: 2, height: 12, color: AppTheme.amber, margin: const EdgeInsets.only(right: 10)),
                      const Text('THINKING',
                          style: TextStyle(
                            color: AppTheme.amber,
                            fontSize: 10,
                            letterSpacing: 3,
                            fontWeight: FontWeight.w700,
                            fontFamilyFallback: AppTheme.monoFallback,
                          )),
                      const SizedBox(width: 8),
                      Text('· $chars 字',
                          style: const TextStyle(
                            color: AppTheme.muted,
                            fontSize: 10,
                            letterSpacing: 1,
                            fontFamilyFallback: AppTheme.monoFallback,
                          )),
                      const Spacer(),
                      _Timestamp(ts: widget.ts),
                      const SizedBox(width: 6),
                      Icon(_expanded ? CupertinoIcons.chevron_up : CupertinoIcons.chevron_down,
                          color: AppTheme.muted, size: 11),
                    ],
                  ),
                  if (_expanded) ...[
                    const SizedBox(height: 10),
                    Container(
                      padding: const EdgeInsets.fromLTRB(12, 8, 8, 8),
                      decoration: const BoxDecoration(
                        border: Border(left: BorderSide(color: AppTheme.amber, width: 2)),
                      ),
                      child: SelectableText(widget.text, style: AppTheme.monoAccent),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _MessageBlock extends StatelessWidget {
  const _MessageBlock({required this.text, required this.ts});
  final String text;
  final DateTime ts;
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('REPORT',
                  style: TextStyle(
                    color: AppTheme.paper,
                    fontSize: 10,
                    letterSpacing: 3,
                    fontWeight: FontWeight.w700,
                    fontFamilyFallback: AppTheme.monoFallback,
                  )),
              const SizedBox(width: 8),
              _Timestamp(ts: ts),
            ],
          ),
          const SizedBox(height: 8),
          MarkdownBody(
            data: text,
            selectable: true,
            onTapLink: (linkText, href, title) async {
              if (href == null || href.isEmpty) return;
              final uri = Uri.tryParse(href);
              if (uri == null) return;
              await launchUrl(uri, mode: LaunchMode.externalApplication);
            },
            styleSheet: MarkdownStyleSheet(
              p: const TextStyle(color: AppTheme.paper, fontSize: 14.5, height: 1.65),
              h1: const TextStyle(color: AppTheme.paper, fontSize: 20, fontWeight: FontWeight.w800, height: 1.4),
              h2: const TextStyle(color: AppTheme.paper, fontSize: 17.5, fontWeight: FontWeight.w800, height: 1.4),
              h3: const TextStyle(color: AppTheme.paper, fontSize: 15.5, fontWeight: FontWeight.w700, height: 1.4),
              listBullet: const TextStyle(color: AppTheme.paper, fontSize: 14.5, height: 1.65),
              strong: const TextStyle(color: AppTheme.paper, fontWeight: FontWeight.w800),
              em: const TextStyle(color: AppTheme.pen, fontStyle: FontStyle.italic),
              a: const TextStyle(color: AppTheme.amber, decoration: TextDecoration.underline),
              code: const TextStyle(
                color: AppTheme.amber,
                backgroundColor: AppTheme.carbon,
                fontSize: 13,
                fontFamilyFallback: AppTheme.monoFallback,
              ),
              codeblockDecoration: BoxDecoration(color: AppTheme.carbon, border: Border.all(color: AppTheme.graphite)),
              codeblockPadding: const EdgeInsets.all(12),
              blockquoteDecoration: const BoxDecoration(
                border: Border(left: BorderSide(color: AppTheme.graphite, width: 3)),
              ),
              blockquotePadding: const EdgeInsets.only(left: 12, top: 4, bottom: 4),
            ),
          ),
        ],
      ),
    );
  }
}

class _ToolCallBlock extends StatelessWidget {
  const _ToolCallBlock({required this.payload, required this.ts});
  final Map<String, dynamic> payload;
  final DateTime ts;
  @override
  Widget build(BuildContext context) {
    final name = (payload['name'] as String?) ?? '';
    final args = (payload['arguments_json'] as String?) ?? '';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          border: Border.all(color: AppTheme.graphite),
          color: AppTheme.carbon,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(CupertinoIcons.arrow_right_circle_fill, color: AppTheme.paper, size: 14),
                const SizedBox(width: 8),
                Text('调用  $name',
                    style: const TextStyle(
                      color: AppTheme.paper,
                      fontSize: 12,
                      letterSpacing: 2,
                      fontWeight: FontWeight.w700,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
                const Spacer(),
                _Timestamp(ts: ts),
              ],
            ),
            if (args.isNotEmpty) ...[
              const SizedBox(height: 8),
              SelectableText(args, style: AppTheme.monoDim, maxLines: 6),
            ],
          ],
        ),
      ),
    );
  }
}

class _ToolResultBlock extends StatelessWidget {
  const _ToolResultBlock({required this.payload, required this.ts});
  final Map<String, dynamic> payload;
  final DateTime ts;
  @override
  Widget build(BuildContext context) {
    final ok = (payload['ok'] as bool?) ?? true;
    final name = (payload['name'] as String?) ?? '';
    final content = (payload['content'] as String?) ?? '';
    return Padding(
      padding: const EdgeInsets.only(top: 0, bottom: 12),
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          border: Border.all(color: ok ? AppTheme.graphite : AppTheme.redline),
          color: AppTheme.carbon,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  ok ? CupertinoIcons.arrow_left_circle_fill : CupertinoIcons.xmark_octagon_fill,
                  color: ok ? AppTheme.sage : AppTheme.redline,
                  size: 14,
                ),
                const SizedBox(width: 8),
                Text(ok ? '回执  $name' : '失败  $name',
                    style: TextStyle(
                      color: ok ? AppTheme.sage : AppTheme.redline,
                      fontSize: 12,
                      letterSpacing: 2,
                      fontWeight: FontWeight.w700,
                      fontFamilyFallback: AppTheme.monoFallback,
                    )),
                const Spacer(),
                _Timestamp(ts: ts),
              ],
            ),
            if (content.isNotEmpty) ...[
              const SizedBox(height: 8),
              SelectableText(content, style: AppTheme.monoDim, maxLines: 12),
            ],
          ],
        ),
      ),
    );
  }
}

class _ArtifactBlock extends StatelessWidget {
  const _ArtifactBlock({required this.payload, required this.ts});
  final Map<String, dynamic> payload;
  final DateTime ts;
  @override
  Widget build(BuildContext context) {
    final name = (payload['name'] as String?) ?? '';
    final size = (payload['size'] as num?)?.toInt() ?? 0;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(border: Border.all(color: AppTheme.sage)),
            child: const Text('VAULT +',
                style: TextStyle(
                  color: AppTheme.sage,
                  fontSize: 10,
                  letterSpacing: 3,
                  fontWeight: FontWeight.w700,
                  fontFamilyFallback: AppTheme.monoFallback,
                )),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text('入柜  $name  · $size B',
                style: const TextStyle(
                  color: AppTheme.paper,
                  fontSize: 12,
                  fontFamilyFallback: AppTheme.monoFallback,
                )),
          ),
          _Timestamp(ts: ts),
        ],
      ),
    );
  }
}

class _SystemBlock extends StatelessWidget {
  const _SystemBlock({required this.payload, required this.ts});
  final Map<String, dynamic> payload;
  final DateTime ts;
  @override
  Widget build(BuildContext context) {
    final kind = (payload['kind'] as String?) ?? 'system';
    final text = (payload['text'] as String?) ?? '';
    Color color;
    String label;
    switch (kind) {
      case 'dispatched':
        color = AppTheme.redline;
        label = 'DISPATCH';
        break;
      case 'task_done':
        color = AppTheme.sage;
        label = 'TASK · DONE';
        break;
      case 'error':
        color = AppTheme.redline;
        label = 'ERROR';
        break;
      case 'max_iter':
        color = AppTheme.amber;
        label = 'MAX ITER';
        break;
      default:
        color = AppTheme.pen;
        label = 'SYS';
    }
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          AppDecor.stamp(label, border: color, color: color),
          const SizedBox(width: 10),
          Expanded(
            child: Text(text,
                style: TextStyle(color: color, fontSize: 12, fontFamilyFallback: AppTheme.monoFallback)),
          ),
          _Timestamp(ts: ts),
        ],
      ),
    );
  }
}

class _EmptyEventState extends StatelessWidget {
  const _EmptyEventState({required this.running});
  final bool running;

  @override
  Widget build(BuildContext context) {
    final waiting = running;
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (waiting) ...[
            const SizedBox(
              height: 26,
              width: 26,
              child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
            ),
            const SizedBox(height: 16),
          ],
          AppDecor.stamp(
            waiting ? 'STANDBY' : 'NO TRACE',
            border: waiting ? AppTheme.amber : AppTheme.pen,
            color: waiting ? AppTheme.amber : AppTheme.pen,
          ),
          const SizedBox(height: 12),
          Text(
            waiting ? '已派遣，正在与代号零接通…' : '本任务没有留下任何痕迹。',
            style: const TextStyle(
              color: AppTheme.pen,
              fontSize: 12,
              letterSpacing: 3,
              fontFamilyFallback: AppTheme.monoFallback,
            ),
          ),
        ],
      ),
    );
  }
}

class _ScrollDownChip extends StatelessWidget {
  const _ScrollDownChip({required this.onTap});
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: AppTheme.carbon,
            border: Border.all(color: AppTheme.paper),
          ),
          child: const Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(CupertinoIcons.arrow_down, size: 12, color: AppTheme.paper),
              SizedBox(width: 4),
              Text('回到最新',
                  style: TextStyle(
                    color: AppTheme.paper,
                    fontSize: 11,
                    letterSpacing: 2,
                    fontFamilyFallback: AppTheme.monoFallback,
                  )),
            ],
          ),
        ),
      ),
    );
  }
}

// ============== 撤离 dialog ==============

class _AbortDialog extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Dialog(
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
      backgroundColor: AppTheme.carbon,
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AppDecor.stamp('CONFIRM', border: AppTheme.redline, color: AppTheme.redline),
            const SizedBox(height: 14),
            const Text(
              '确认撤离？',
              style: TextStyle(color: AppTheme.paper, fontSize: 18, fontWeight: FontWeight.w700, letterSpacing: 2),
            ),
            const SizedBox(height: 8),
            const Text(
              '撤离后任务立刻终止；已经写入工件柜的产出会保留，'
              '但未完成的思考会丢失。',
              style: TextStyle(color: AppTheme.pen, fontSize: 13, height: 1.5),
            ),
            const SizedBox(height: 18),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                OutlinedButton(onPressed: () => Navigator.pop(context, false), child: const Text('继续行动')),
                const SizedBox(width: 8),
                FilledButton(
                  style: FilledButton.styleFrom(backgroundColor: AppTheme.redline),
                  onPressed: () => Navigator.pop(context, true),
                  child: const Text('撤离'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// ============== 工件柜（底部抽屉） ==============

class _VaultSheet extends StatelessWidget {
  const _VaultSheet({required this.artifacts});
  final List<Artifact> artifacts;

  @override
  Widget build(BuildContext context) {
    return DraggableScrollableSheet(
      initialChildSize: 0.6,
      minChildSize: 0.3,
      maxChildSize: 0.92,
      expand: false,
      builder: (ctx, scroll) {
        return Container(
          color: AppTheme.carbon,
          child: Column(
            children: [
              Container(
                margin: const EdgeInsets.only(top: 10, bottom: 4),
                width: 36,
                height: 3,
                decoration: BoxDecoration(color: AppTheme.graphite, borderRadius: BorderRadius.circular(2)),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 10),
                child: Row(
                  children: [
                    AppDecor.stamp('VAULT', border: AppTheme.sage, color: AppTheme.sage),
                    const SizedBox(width: 10),
                    const Text('工件柜',
                        style: TextStyle(
                          color: AppTheme.paper,
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                          letterSpacing: 4,
                        )),
                    const Spacer(),
                    Text('${artifacts.length}',
                        style: const TextStyle(
                          color: AppTheme.pen,
                          fontSize: 12,
                          letterSpacing: 2,
                          fontFamilyFallback: AppTheme.monoFallback,
                        )),
                  ],
                ),
              ),
              const Divider(height: 1, color: AppTheme.graphite),
              Expanded(
                child: artifacts.isEmpty
                    ? const Center(
                        child: Text(
                          '柜中无物。\n让特工调用 笔录(write_file) 入柜产出。',
                          textAlign: TextAlign.center,
                          style: TextStyle(color: AppTheme.muted, fontSize: 12, height: 1.6),
                        ),
                      )
                    : ListView.separated(
                        controller: scroll,
                        padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
                        itemBuilder: (_, i) => _ArtifactRow(
                          artifact: artifacts[i],
                          onTap: () {
                            final a = artifacts[i];
                            Navigator.of(ctx).pop();
                            ctx.push(
                              '/missions/${a.missionId}/artifacts/${a.id}'
                              '?name=${Uri.encodeQueryComponent(a.name)}'
                              '&mime=${Uri.encodeQueryComponent(a.mime)}',
                            );
                          },
                        ),
                        separatorBuilder: (_, _) => const Divider(color: AppTheme.graphite),
                        itemCount: artifacts.length,
                      ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _ArtifactRow extends StatelessWidget {
  const _ArtifactRow({required this.artifact, required this.onTap});
  final Artifact artifact;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 10),
        child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            alignment: Alignment.center,
            decoration: BoxDecoration(border: Border.all(color: AppTheme.graphite)),
            child: const Icon(CupertinoIcons.doc_text, color: AppTheme.paper, size: 16),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(artifact.name,
                    style: const TextStyle(
                      color: AppTheme.paper,
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 1,
                    )),
                const SizedBox(height: 2),
                Text(
                  '${artifact.mime.isEmpty ? "file" : artifact.mime} · ${artifact.size} B',
                  style: const TextStyle(
                    color: AppTheme.muted,
                    fontSize: 11,
                    letterSpacing: 1,
                    fontFamilyFallback: AppTheme.monoFallback,
                  ),
                ),
              ],
            ),
          ),
          Text(DateFormat('MM/dd HH:mm').format(artifact.createdAt),
              style: const TextStyle(
                color: AppTheme.muted,
                fontSize: 10,
                letterSpacing: 1,
                fontFamilyFallback: AppTheme.monoFallback,
              )),
          const SizedBox(width: 6),
          const Icon(CupertinoIcons.chevron_right, color: AppTheme.muted, size: 12),
        ],
        ),
      ),
    );
  }
}

// 让 Mission 有一个 copy 帮助方法（仅供本页面内部更新 status 用）。
extension on Mission {
  Mission copyWithStatus(MissionStatus s) => Mission(
        id: id,
        codename: codename,
        brief: brief,
        tier: tier,
        status: s,
        loadout: loadout,
        inputTokens: inputTokens,
        outputTokens: outputTokens,
        createdAt: createdAt,
        startedAt: startedAt,
        endedAt: endedAt,
      );
}

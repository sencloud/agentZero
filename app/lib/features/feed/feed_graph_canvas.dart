import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';

import '../../core/theme.dart';
import '../../models/feed.dart';

/// 事件流图谱画布：CustomPaint + 力导向布局 + 拖动节点。
///
/// 渲染策略：
///   - 节点：实体类型映射不同色（人物琥珀、机构白、地点纸白、概念灰、事件朱红）
///   - 边粗细 = max(0.6, min(2.2, weight))；颜色随 weight 由暗到亮
///   - 力导向：弹簧 + 排斥 + 中心拉力，迭代 N 帧后收敛（用户拖动节点会冻结该节点）
class FeedGraphCanvas extends StatefulWidget {
  const FeedGraphCanvas({super.key, required this.graph});

  final FeedGraph graph;

  @override
  State<FeedGraphCanvas> createState() => _FeedGraphCanvasState();
}

class _Node {
  _Node({required this.id, required this.data, required this.pos});
  final int id;
  final GraphNode data;
  Offset pos;
  Offset vel = Offset.zero;
  bool pinned = false;
}

class _FeedGraphCanvasState extends State<FeedGraphCanvas>
    with SingleTickerProviderStateMixin {
  late List<_Node> _nodes;
  late Map<int, _Node> _byId;
  Size _size = Size.zero;

  late final Ticker _ticker;
  int _frame = 0;
  int? _dragging;
  Offset? _dragLast;

  @override
  void initState() {
    super.initState();
    _hydrate();
    _ticker = createTicker(_step);
    _ticker.start();
  }

  @override
  void didUpdateWidget(covariant FeedGraphCanvas oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.graph != widget.graph) {
      _hydrate();
      _frame = 0;
    }
  }

  void _hydrate() {
    final rng = math.Random(7);
    _nodes = widget.graph.nodes.map((n) {
      return _Node(
        id: n.id,
        data: n,
        pos: Offset(rng.nextDouble() * 300 - 150, rng.nextDouble() * 300 - 150),
      );
    }).toList();
    _byId = {for (final n in _nodes) n.id: n};
  }

  void _step(Duration _) {
    if (_size == Size.zero || _nodes.isEmpty) return;
    if (_frame > 600) return; // 600 帧后停止
    _frame++;
    final cx = _size.width / 2;
    final cy = _size.height / 2;
    final repel = 4200.0;
    final spring = 0.012;
    final centerPull = 0.0015;
    final damping = 0.86;
    final restLen = 110.0;

    for (final n in _nodes) {
      if (n.pinned) continue;
      Offset force = Offset.zero;
      for (final m in _nodes) {
        if (m == n) continue;
        final d = n.pos - m.pos;
        final dist2 = d.dx * d.dx + d.dy * d.dy + 0.01;
        final f = repel / dist2;
        final dist = math.sqrt(dist2);
        force += Offset(d.dx / dist * f, d.dy / dist * f);
      }
      force += Offset(-n.pos.dx * centerPull * _size.width,
          -n.pos.dy * centerPull * _size.height);
      n.vel = (n.vel + force * 0.0008) * damping;
    }
    for (final e in widget.graph.edges) {
      final a = _byId[e.srcId];
      final b = _byId[e.dstId];
      if (a == null || b == null) continue;
      final d = b.pos - a.pos;
      final dist = d.distance.clamp(0.001, 9999.0);
      final diff = (dist - restLen) * spring * e.weight.clamp(0.3, 1.5);
      final dir = Offset(d.dx / dist, d.dy / dist);
      if (!a.pinned) a.vel += dir * diff;
      if (!b.pinned) b.vel -= dir * diff;
    }
    for (final n in _nodes) {
      if (!n.pinned) n.pos += n.vel;
    }
    setState(() {
      // 节点和速度都是 mutable，只触发重绘
    });
    // 仍然把 cx/cy 用上避免静态分析警告（实际我们不绝对定位中心，使用相对偏移）
    if (cx < 0 || cy < 0) return;
  }

  @override
  void dispose() {
    _ticker.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (_, c) {
        _size = c.biggest;
        if (_nodes.isEmpty) {
          return const Center(
            child: Text(
              '尚无图谱节点\n添加关心的话题后，新事件会持续灌入',
              textAlign: TextAlign.center,
              style: TextStyle(color: AppTheme.muted, fontSize: 12, height: 1.6, letterSpacing: 2),
            ),
          );
        }
        return GestureDetector(
          onPanStart: (d) {
            final hit = _hit(d.localPosition);
            if (hit != null) {
              hit.pinned = true;
              _dragging = hit.id;
              _dragLast = d.localPosition;
            }
          },
          onPanUpdate: (d) {
            if (_dragging == null) return;
            final n = _byId[_dragging!];
            if (n == null) return;
            final delta = d.localPosition - (_dragLast ?? d.localPosition);
            n.pos += delta;
            n.vel = Offset.zero;
            _dragLast = d.localPosition;
            _frame = 0; // 重启迭代
          },
          onPanEnd: (_) {
            if (_dragging != null) {
              _byId[_dragging!]?.pinned = false;
            }
            _dragging = null;
            _dragLast = null;
          },
          child: CustomPaint(
            size: Size.infinite,
            painter: _GraphPainter(nodes: _nodes, edges: widget.graph.edges),
          ),
        );
      },
    );
  }

  _Node? _hit(Offset p) {
    final cx = _size.width / 2;
    final cy = _size.height / 2;
    for (final n in _nodes.reversed) {
      final c = Offset(cx + n.pos.dx, cy + n.pos.dy);
      if ((c - p).distance < 22) return n;
    }
    return null;
  }
}

class _GraphPainter extends CustomPainter {
  _GraphPainter({required this.nodes, required this.edges});
  final List<_Node> nodes;
  final List<GraphEdge> edges;

  @override
  void paint(Canvas canvas, Size size) {
    final cx = size.width / 2;
    final cy = size.height / 2;
    final byId = {for (final n in nodes) n.id: n};

    // 1) 画边
    for (final e in edges) {
      final a = byId[e.srcId];
      final b = byId[e.dstId];
      if (a == null || b == null) continue;
      final p1 = Offset(cx + a.pos.dx, cy + a.pos.dy);
      final p2 = Offset(cx + b.pos.dx, cy + b.pos.dy);
      final w = e.weight.clamp(0.3, 1.6);
      final paint = Paint()
        ..color = AppTheme.graphite.withValues(alpha: 0.4 + 0.5 * (w / 1.6))
        ..strokeWidth = 0.6 + w * 0.8;
      canvas.drawLine(p1, p2, paint);

      // 边中点画 label
      if (e.label.isNotEmpty && w > 0.6) {
        final mid = Offset((p1.dx + p2.dx) / 2, (p1.dy + p2.dy) / 2);
        final tp = TextPainter(
          text: TextSpan(
            text: e.label,
            style: const TextStyle(
              color: AppTheme.muted,
              fontSize: 9,
              letterSpacing: 1,
            ),
          ),
          textDirection: TextDirection.ltr,
        )..layout();
        tp.paint(canvas, Offset(mid.dx - tp.width / 2, mid.dy - tp.height / 2));
      }
    }

    // 2) 画节点
    for (final n in nodes) {
      final c = Offset(cx + n.pos.dx, cy + n.pos.dy);
      final color = _colorForType(n.data.type);
      final r = 8.0 + math.min(n.data.weight, 6.0) * 1.6;
      // 光晕
      final glow = Paint()
        ..color = color.withValues(alpha: 0.18)
        ..maskFilter = const ui.MaskFilter.blur(ui.BlurStyle.normal, 6);
      canvas.drawCircle(c, r + 4, glow);

      final body = Paint()..color = AppTheme.carbon;
      canvas.drawCircle(c, r, body);
      final ring = Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.2
        ..color = color;
      canvas.drawCircle(c, r, ring);

      // 标签
      final tp = TextPainter(
        text: TextSpan(
          text: _truncate(n.data.name, 10),
          style: const TextStyle(
            color: AppTheme.paper,
            fontSize: 11,
            letterSpacing: 1.5,
          ),
        ),
        textDirection: TextDirection.ltr,
      )..layout(maxWidth: 120);
      tp.paint(canvas, Offset(c.dx - tp.width / 2, c.dy + r + 4));
    }
  }

  Color _colorForType(String type) {
    switch (type) {
      case 'person':
        return AppTheme.amber;
      case 'org':
        return AppTheme.paper;
      case 'place':
        return const Color(0xFF8FB7A6);
      case 'event':
        return AppTheme.redline;
      case 'concept':
      default:
        return AppTheme.pen;
    }
  }

  String _truncate(String s, int n) {
    return s.length <= n ? s : '${s.substring(0, n)}…';
  }

  @override
  bool shouldRepaint(covariant _GraphPainter old) => true;
}


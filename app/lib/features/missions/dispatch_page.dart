import 'dart:math';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/theme.dart';
import '../../models/mission.dart';
import '../../providers/missions.dart';

/// 派遣页（M3c）。
///
/// 用户在这里写下：
///   - 行动代号（可随机）
///   - 任务简报（必填，自由文本）
///   - 档位（三选一，单选）
///   - 携带装备（从 /tools 拉，至少勾一个）
///
/// 提交后调用 POST /missions，成功跳转到行动现场。
class DispatchPage extends ConsumerStatefulWidget {
  const DispatchPage({super.key});
  @override
  ConsumerState<DispatchPage> createState() => _DispatchPageState();
}

class _DispatchPageState extends ConsumerState<DispatchPage> {
  final _codenameCtrl = TextEditingController();
  final _briefCtrl = TextEditingController();
  MissionTier _tier = MissionTier.standard;
  final Set<String> _selectedTools = {};

  bool _submitting = false;
  String? _error;

  // 代号池：派遣页可随机抽。具备特工感的两字 / 三字短代号。
  static const _codenames = <String>[
    '北极星', '猎户', '苍鹰', '破晓', '夜枭', '潮汐', '孤鸢', '白蚁',
    '海狼', '残月', '回声', '晨曦', '落雷', '黑燕', '钨钢', '青鸟',
    '断弦', '雪原', '电光', '雾岛', '寒星', '霜叶', '荒原', '裂帛',
  ];

  @override
  void initState() {
    super.initState();
    _codenameCtrl.text = _codenames[Random().nextInt(_codenames.length)];
  }

  @override
  void dispose() {
    _codenameCtrl.dispose();
    _briefCtrl.dispose();
    super.dispose();
  }

  void _shuffleCodename() {
    setState(() {
      _codenameCtrl.text = _codenames[Random().nextInt(_codenames.length)];
    });
  }

  Future<void> _dispatch() async {
    final codename = _codenameCtrl.text.trim();
    final brief = _briefCtrl.text.trim();
    if (brief.isEmpty) {
      setState(() => _error = '任务简报不能为空');
      return;
    }
    if (_selectedTools.isEmpty) {
      setState(() => _error = '至少携带一件装备');
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      final mission = await ref.read(dispatchMissionProvider).call(
            codename: codename.isEmpty ? '未命名行动' : codename,
            brief: brief,
            tier: _tier,
            loadout: _selectedTools.toList(),
          );
      if (mounted) {
        context.go('/missions/${mission.id}');
      }
    } catch (e) {
      if (mounted) {
        final msg = '派遣失败：$e';
        setState(() {
          _submitting = false;
          _error = msg;
        });
        // 同时弹一条 snackbar，防止用户没注意到内嵌错误条
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            backgroundColor: AppTheme.redline,
            content: Text(msg, style: const TextStyle(color: AppTheme.paper)),
            duration: const Duration(seconds: 5),
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final toolsAsync = ref.watch(toolsProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('派遣行动'),
        leading: IconButton(
          icon: const Icon(CupertinoIcons.xmark),
          onPressed: () => context.pop(),
        ),
      ),
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: ListView(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
                children: [
                  AppDecor.sectionRule('行动代号'),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      Expanded(
                        child: TextField(
                          controller: _codenameCtrl,
                          maxLength: 12,
                          style: const TextStyle(
                            color: AppTheme.paper,
                            fontSize: 22,
                            fontWeight: FontWeight.w700,
                            letterSpacing: 4,
                          ),
                          decoration: const InputDecoration(
                            counterText: '',
                            hintText: '给这次行动起个代号',
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      OutlinedButton(
                        onPressed: _shuffleCodename,
                        child: const Text('随机'),
                      ),
                    ],
                  ),
                  const SizedBox(height: 20),
                  AppDecor.sectionRule('任务简报'),
                  const SizedBox(height: 8),
                  TextField(
                    controller: _briefCtrl,
                    minLines: 5,
                    maxLines: 14,
                    style: const TextStyle(color: AppTheme.paper, fontSize: 14, height: 1.6),
                    decoration: const InputDecoration(
                      hintText: '用中文描述你要让代号零完成什么：研究 / 写作 / 抓资料 / 整理 / 生成报告……\n越具体越好，可以写多段。',
                    ),
                  ),
                  const SizedBox(height: 20),
                  AppDecor.sectionRule('档位'),
                  const SizedBox(height: 8),
                  _TierPicker(value: _tier, onChanged: (t) => setState(() => _tier = t)),
                  const SizedBox(height: 20),
                  AppDecor.sectionRule('装备 · loadout'),
                  const SizedBox(height: 8),
                  toolsAsync.when(
                    loading: () => const Padding(
                      padding: EdgeInsets.symmetric(vertical: 24),
                      child: Center(
                        child: SizedBox(
                          height: 22,
                          width: 22,
                          child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
                        ),
                      ),
                    ),
                    error: (e, _) => Text('装备列表加载失败：$e',
                        style: const TextStyle(color: AppTheme.redline, fontSize: 12)),
                    data: (tools) {
                      return Column(
                        children: [
                          for (final t in tools)
                            _ToolRow(
                              tool: t,
                              selected: _selectedTools.contains(t.name),
                              onTap: () {
                                setState(() {
                                  if (_selectedTools.contains(t.name)) {
                                    _selectedTools.remove(t.name);
                                  } else {
                                    _selectedTools.add(t.name);
                                  }
                                });
                              },
                            ),
                        ],
                      );
                    },
                  ),
                  if (_error != null) ...[
                    const SizedBox(height: 16),
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(border: Border.all(color: AppTheme.redline)),
                      child: Row(
                        children: [
                          const Icon(CupertinoIcons.exclamationmark_triangle,
                              color: AppTheme.redline, size: 14),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(_error!,
                                style: const TextStyle(color: AppTheme.redline, fontSize: 12)),
                          ),
                        ],
                      ),
                    ),
                  ],
                ],
              ),
            ),
            // 底部派遣按钮区
            Container(
              padding: const EdgeInsets.fromLTRB(20, 14, 20, 14),
              decoration: const BoxDecoration(
                color: AppTheme.ink,
                border: Border(top: BorderSide(color: AppTheme.graphite)),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      '${_selectedTools.length} 件装备  ·  ${_tier.label}',
                      style: const TextStyle(
                        color: AppTheme.pen,
                        fontSize: 11,
                        letterSpacing: 2,
                        fontFamilyFallback: AppTheme.monoFallback,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: _submitting ? null : _dispatch,
                    child: _submitting
                        ? const SizedBox(
                            height: 18,
                            width: 18,
                            child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.paper),
                          )
                        : const Text('派遣 →'),
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

class _TierPicker extends StatelessWidget {
  const _TierPicker({required this.value, required this.onChanged});
  final MissionTier value;
  final ValueChanged<MissionTier> onChanged;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        for (final t in MissionTier.values)
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(right: t == MissionTier.pro ? 0 : 8),
              child: _TierTile(
                tier: t,
                selected: value == t,
                onTap: () => onChanged(t),
              ),
            ),
          ),
      ],
    );
  }
}

class _TierTile extends StatelessWidget {
  const _TierTile({required this.tier, required this.selected, required this.onTap});
  final MissionTier tier;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final border = selected ? AppTheme.redline : AppTheme.graphite;
    return InkWell(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 10),
        decoration: BoxDecoration(
          border: Border.all(color: border, width: selected ? 1.2 : 0.8),
          color: selected ? AppTheme.redlineDim.withValues(alpha: 0.18) : AppTheme.carbon,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              tier.label,
              style: TextStyle(
                color: selected ? AppTheme.paper : AppTheme.pen,
                fontSize: 14,
                fontWeight: FontWeight.w800,
                letterSpacing: 2,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              tier.desc,
              style: const TextStyle(
                color: AppTheme.muted,
                fontSize: 10,
                letterSpacing: 1,
                fontFamilyFallback: AppTheme.monoFallback,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ToolRow extends StatelessWidget {
  const _ToolRow({required this.tool, required this.selected, required this.onTap});
  final ToolInfo tool;
  final bool selected;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: InkWell(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            border: Border.all(color: selected ? AppTheme.paper : AppTheme.graphite, width: selected ? 1.0 : 0.8),
            color: AppTheme.carbon,
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 18,
                height: 18,
                decoration: BoxDecoration(
                  border: Border.all(color: selected ? AppTheme.redline : AppTheme.pen, width: 1),
                  color: selected ? AppTheme.redline : Colors.transparent,
                ),
                child: selected
                    ? const Icon(CupertinoIcons.check_mark, size: 12, color: AppTheme.paper)
                    : null,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(
                          tool.displayName,
                          style: const TextStyle(
                            color: AppTheme.paper,
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            letterSpacing: 2,
                          ),
                        ),
                        const SizedBox(width: 8),
                        Text(
                          tool.name,
                          style: const TextStyle(
                            color: AppTheme.muted,
                            fontSize: 10,
                            letterSpacing: 1,
                            fontFamilyFallback: AppTheme.monoFallback,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(
                      tool.description,
                      style: const TextStyle(color: AppTheme.pen, fontSize: 12, height: 1.5),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

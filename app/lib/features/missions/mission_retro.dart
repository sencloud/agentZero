import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/theme.dart';
import '../../models/mission.dart';
import '../../providers/missions.dart';

/// 行动复盘卡：任务终态后出现在事件流末尾。
/// - 星级评分（点击单颗即可即时提交）
/// - 评语 / 改评
/// - 「继续安排」入口
class MissionRetroCard extends ConsumerWidget {
  const MissionRetroCard({super.key, required this.mission});
  final Mission mission;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final reviewAsync = ref.watch(missionReviewProvider(mission.id));
    return Container(
      margin: const EdgeInsets.only(top: 20, bottom: 40),
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
      decoration: BoxDecoration(
        color: AppTheme.carbon,
        border: Border.all(color: AppTheme.graphite),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              AppDecor.stamp('AFTER ACTION', border: AppTheme.amber, color: AppTheme.amber),
              const SizedBox(width: 10),
              const Text(
                '行动复盘',
                style: TextStyle(
                  color: AppTheme.paper,
                  fontSize: 13,
                  letterSpacing: 4,
                  fontWeight: FontWeight.w700,
                  fontFamilyFallback: AppTheme.monoFallback,
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          reviewAsync.when(
            loading: () => const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: Center(
                child: SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTheme.amber),
                ),
              ),
            ),
            error: (e, _) => _ErrText(text: '点评加载失败：$e'),
            data: (review) => _RatingRow(mission: mission, review: review),
          ),
          const SizedBox(height: 14),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: () => _openFollowUp(context, ref),
                  icon: const Icon(CupertinoIcons.arrow_right_to_line, size: 16, color: AppTheme.paper),
                  label: const Text('继续安排'),
                  style: OutlinedButton.styleFrom(
                    side: const BorderSide(color: AppTheme.paper, width: 1),
                    foregroundColor: AppTheme.paper,
                    shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  void _openFollowUp(BuildContext context, WidgetRef ref) {
    context.push('/missions/new?parent_id=${Uri.encodeQueryComponent(mission.id)}');
  }
}

class _RatingRow extends ConsumerStatefulWidget {
  const _RatingRow({required this.mission, required this.review});
  final Mission mission;
  final MissionReview? review;

  @override
  ConsumerState<_RatingRow> createState() => _RatingRowState();
}

class _RatingRowState extends ConsumerState<_RatingRow> {
  int _hover = 0;
  bool _submitting = false;
  final TextEditingController _commentCtrl = TextEditingController();
  bool _editingComment = false;

  @override
  void initState() {
    super.initState();
    _commentCtrl.text = widget.review?.comment ?? '';
  }

  @override
  void didUpdateWidget(covariant _RatingRow oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.review?.comment != oldWidget.review?.comment && !_editingComment) {
      _commentCtrl.text = widget.review?.comment ?? '';
    }
  }

  @override
  void dispose() {
    _commentCtrl.dispose();
    super.dispose();
  }

  int get _currentRating => widget.review?.rating ?? 0;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            for (var i = 1; i <= 5; i++)
              GestureDetector(
                onTap: _submitting ? null : () => _submit(i),
                onTapDown: (_) => setState(() => _hover = i),
                onTapCancel: () => setState(() => _hover = 0),
                child: Padding(
                  padding: const EdgeInsets.only(right: 6),
                  child: Icon(
                    (_hover > 0 ? _hover : _currentRating) >= i
                        ? CupertinoIcons.star_fill
                        : CupertinoIcons.star,
                    color: AppTheme.amber,
                    size: 26,
                  ),
                ),
              ),
            const SizedBox(width: 10),
            if (_submitting)
              const SizedBox(
                width: 14,
                height: 14,
                child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTheme.amber),
              )
            else if (_currentRating > 0)
              Text('$_currentRating / 5',
                  style: const TextStyle(
                    color: AppTheme.amber,
                    fontSize: 12,
                    letterSpacing: 2,
                    fontFamilyFallback: AppTheme.monoFallback,
                  ))
            else
              const Text('点星即评',
                  style: TextStyle(
                    color: AppTheme.muted,
                    fontSize: 12,
                    letterSpacing: 2,
                    fontFamilyFallback: AppTheme.monoFallback,
                  )),
          ],
        ),
        const SizedBox(height: 10),
        TextField(
          controller: _commentCtrl,
          maxLines: 3,
          minLines: 1,
          maxLength: 200,
          style: const TextStyle(color: AppTheme.paper, fontSize: 13),
          decoration: const InputDecoration(
            hintText: '一句话点评（可选）',
            counterText: '',
          ),
          onChanged: (_) => _editingComment = true,
          onSubmitted: (_) {
            if (_currentRating > 0) _submit(_currentRating, force: true);
          },
        ),
        if (_editingComment && _currentRating > 0)
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              onPressed: _submitting ? null : () => _submit(_currentRating, force: true),
              child: const Text('保存评语'),
            ),
          ),
      ],
    );
  }

  Future<void> _submit(int rating, {bool force = false}) async {
    if (rating == _currentRating && !force) return;
    setState(() => _submitting = true);
    try {
      await ref.read(submitReviewProvider).call(
            missionId: widget.mission.id,
            rating: rating,
            comment: _commentCtrl.text.trim(),
          );
      _editingComment = false;
      if (mounted && rating >= 4) {
        await _maybeDistillSkill(context, ref, widget.mission, rating);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            backgroundColor: AppTheme.redline,
            content: Text('评分提交失败：$e', style: const TextStyle(color: AppTheme.paper)),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }
}

/// 高分行动后弹"是否沉淀为 Skill"。用户选"沉淀"则进入编辑表单。
Future<void> _maybeDistillSkill(
  BuildContext context,
  WidgetRef ref,
  Mission mission,
  int rating,
) async {
  final yes = await showDialog<bool>(
    context: context,
    builder: (ctx) => Dialog(
      backgroundColor: AppTheme.carbon,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AppDecor.stamp('SKILL?', border: AppTheme.amber, color: AppTheme.amber),
            const SizedBox(height: 14),
            const Text('高分行动 · 是否提炼为可复用技能？',
                style: TextStyle(
                    color: AppTheme.paper,
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 2)),
            const SizedBox(height: 8),
            Text(
              '你给了 $rating 星，意味着这次行动的方法论值得沉淀。\n点「沉淀技能」后填写一个名字和触发提示，下次类似任务可以让代号零直接套用。',
              style: const TextStyle(color: AppTheme.pen, fontSize: 13, height: 1.5),
            ),
            const SizedBox(height: 18),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('暂不')),
                const SizedBox(width: 8),
                FilledButton(
                  style: FilledButton.styleFrom(backgroundColor: AppTheme.amber, foregroundColor: AppTheme.ink),
                  onPressed: () => Navigator.pop(ctx, true),
                  child: const Text('沉淀技能'),
                ),
              ],
            ),
          ],
        ),
      ),
    ),
  );
  if (yes != true || !context.mounted) return;
  await showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    backgroundColor: AppTheme.ink,
    builder: (ctx) => _SkillForm(mission: mission),
  );
}

class _SkillForm extends ConsumerStatefulWidget {
  const _SkillForm({required this.mission});
  final Mission mission;

  @override
  ConsumerState<_SkillForm> createState() => _SkillFormState();
}

class _SkillFormState extends ConsumerState<_SkillForm> {
  late final TextEditingController _name = TextEditingController(text: widget.mission.codename);
  final TextEditingController _desc = TextEditingController();
  final TextEditingController _trigger = TextEditingController();
  late final TextEditingController _prompt = TextEditingController(text: widget.mission.brief);
  bool _saving = false;

  @override
  void dispose() {
    _name.dispose();
    _desc.dispose();
    _trigger.dispose();
    _prompt.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final bottom = MediaQuery.of(context).viewInsets.bottom;
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 18, 20, 20 + bottom),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              AppDecor.stamp('SKILL', border: AppTheme.amber, color: AppTheme.amber),
              const SizedBox(width: 10),
              const Text('技能沉淀',
                  style: TextStyle(
                      color: AppTheme.paper,
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 4)),
              const Spacer(),
              IconButton(
                icon: const Icon(CupertinoIcons.xmark, color: AppTheme.paper, size: 18),
                onPressed: () => Navigator.pop(context),
              ),
            ],
          ),
          const SizedBox(height: 12),
          _label('技能名称'),
          TextField(controller: _name, maxLength: 40, decoration: const InputDecoration(counterText: '')),
          const SizedBox(height: 10),
          _label('一句话描述'),
          TextField(controller: _desc, maxLength: 80, decoration: const InputDecoration(counterText: '')),
          const SizedBox(height: 10),
          _label('启用时机（什么任务可以套用）'),
          TextField(controller: _trigger, maxLength: 80, decoration: const InputDecoration(counterText: '')),
          const SizedBox(height: 10),
          _label('Prompt 模板 / 方法要点'),
          TextField(
            controller: _prompt,
            maxLines: 5,
            minLines: 3,
            decoration: const InputDecoration(),
          ),
          const SizedBox(height: 16),
          FilledButton(
            onPressed: _saving ? null : _save,
            style: FilledButton.styleFrom(
              backgroundColor: AppTheme.amber,
              foregroundColor: AppTheme.ink,
              shape: const RoundedRectangleBorder(borderRadius: BorderRadius.zero),
              minimumSize: const Size.fromHeight(46),
            ),
            child: _saving
                ? const SizedBox(
                    width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTheme.ink))
                : const Text('入档技能'),
          ),
        ],
      ),
    );
  }

  Widget _label(String s) => Padding(
        padding: const EdgeInsets.only(bottom: 4),
        child: Text(s,
            style: const TextStyle(
              color: AppTheme.amber,
              fontSize: 10,
              letterSpacing: 3,
              fontWeight: FontWeight.w700,
              fontFamilyFallback: AppTheme.monoFallback,
            )),
      );

  Future<void> _save() async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('技能名称必填'), backgroundColor: AppTheme.redline),
      );
      return;
    }
    setState(() => _saving = true);
    try {
      await ref.read(createSkillProvider).call(
            name: name,
            description: _desc.text.trim(),
            triggerHint: _trigger.text.trim(),
            promptTemplate: _prompt.text.trim(),
            sourceMissionId: widget.mission.id,
          );
      if (mounted) {
        Navigator.pop(context);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('技能已入档'),
            backgroundColor: AppTheme.ink,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('保存失败：$e'), backgroundColor: AppTheme.redline),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }
}

class _ErrText extends StatelessWidget {
  const _ErrText({required this.text});
  final String text;
  @override
  Widget build(BuildContext context) {
    return Text(text, style: const TextStyle(color: AppTheme.redline, fontSize: 12));
  }
}

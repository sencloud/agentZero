import 'dart:io';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_html/flutter_html.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart' show getTemporaryDirectory;
import 'package:share_plus/share_plus.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../core/theme.dart';
import '../../providers/missions.dart';

/// 工件预览页：
///   - text/html  → 用 flutter_html 渲染（链接 tap 调外部浏览器）
///   - text/*     → 选择性纯文本展示
///   - 其它       → 显示元数据 + 提示不支持
///
/// 顶栏右上角：复制全文 / 分享（写到临时文件后调 share_plus）。
class ArtifactViewerPage extends ConsumerWidget {
  const ArtifactViewerPage({
    super.key,
    required this.missionId,
    required this.artifactId,
    required this.artifactName,
    required this.artifactMime,
  });

  final String missionId;
  final int artifactId;
  final String artifactName;
  final String artifactMime;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncContent = watchArtifactContent(ref, missionId, artifactId);

    return Scaffold(
      backgroundColor: AppTheme.paper,
      appBar: AppBar(
        backgroundColor: AppTheme.ink,
        title: Text(
          artifactName,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(
            color: AppTheme.paper,
            fontSize: 14,
            letterSpacing: 2,
            fontWeight: FontWeight.w700,
          ),
        ),
        actions: [
          asyncContent.maybeWhen(
            data: (c) => _ActionButtons(content: c, name: artifactName),
            orElse: () => const SizedBox.shrink(),
          ),
        ],
      ),
      body: asyncContent.when(
        loading: () => const Center(
          child: SizedBox(
            height: 26,
            width: 26,
            child: CircularProgressIndicator(strokeWidth: 2, color: AppTheme.ink),
          ),
        ),
        error: (e, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Text(
              '加载失败：$e',
              style: const TextStyle(color: AppTheme.redline, fontSize: 12),
            ),
          ),
        ),
        data: (c) {
          if (c.isHtml) return _HtmlView(html: c.text);
          if (c.isText) return _TextView(text: c.text);
          return _UnsupportedView(mime: c.mime, size: c.bytes.length);
        },
      ),
    );
  }
}

class _HtmlView extends StatelessWidget {
  const _HtmlView({required this.html});
  final String html;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppTheme.paper,
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 32),
        child: Html(
          data: html,
          onLinkTap: (url, attributes, element) async {
            if (url == null) return;
            final uri = Uri.tryParse(url);
            if (uri == null) return;
            await launchUrl(uri, mode: LaunchMode.externalApplication);
          },
          style: {
            // flutter_html 默认会用 system 字体，调整下整体观感
            'body': Style(
              backgroundColor: AppTheme.paper,
              color: AppTheme.ink,
              fontSize: FontSize(16),
              lineHeight: const LineHeight(1.7),
              margin: Margins.zero,
              padding: HtmlPaddings.zero,
            ),
            'h1': Style(
              color: AppTheme.ink,
              fontSize: FontSize(22),
              fontWeight: FontWeight.w800,
              margin: Margins.only(top: 18, bottom: 10),
            ),
            'h2': Style(
              color: AppTheme.ink,
              fontSize: FontSize(19),
              fontWeight: FontWeight.w700,
              margin: Margins.only(top: 16, bottom: 8),
            ),
            'h3': Style(
              color: AppTheme.ink,
              fontSize: FontSize(16.5),
              fontWeight: FontWeight.w700,
              margin: Margins.only(top: 14, bottom: 6),
            ),
            'a': Style(
              color: const Color(0xFFB80003),
              textDecoration: TextDecoration.underline,
            ),
            'code': Style(
              backgroundColor: const Color(0xFFF1EFEA),
              padding: HtmlPaddings.symmetric(horizontal: 4, vertical: 1),
            ),
            'pre': Style(
              backgroundColor: const Color(0xFFF1EFEA),
              padding: HtmlPaddings.all(12),
              margin: Margins.symmetric(vertical: 10),
            ),
            'blockquote': Style(
              border: const Border(left: BorderSide(color: Color(0xFFB80003), width: 3)),
              padding: HtmlPaddings.only(left: 12),
              margin: Margins.symmetric(vertical: 10),
              color: const Color(0xFF555555),
            ),
            'table': Style(
              border: Border.all(color: const Color(0xFFCFC9BE)),
            ),
            'th, td': Style(
              padding: HtmlPaddings.all(8),
              border: Border.all(color: const Color(0xFFCFC9BE)),
            ),
            'th': Style(
              backgroundColor: const Color(0xFFF1EFEA),
              fontWeight: FontWeight.w700,
            ),
            'ul, ol': Style(margin: Margins.only(left: 4, top: 6, bottom: 10)),
            'li': Style(margin: Margins.only(bottom: 4)),
          },
        ),
      ),
    );
  }
}

class _TextView extends StatelessWidget {
  const _TextView({required this.text});
  final String text;
  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppTheme.paper,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: SelectableText(
          text,
          style: const TextStyle(
            color: AppTheme.ink,
            fontSize: 14,
            height: 1.65,
            fontFamilyFallback: AppTheme.monoFallback,
          ),
        ),
      ),
    );
  }
}

class _UnsupportedView extends StatelessWidget {
  const _UnsupportedView({required this.mime, required this.size});
  final String mime;
  final int size;
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(CupertinoIcons.doc, size: 48, color: AppTheme.ink),
            const SizedBox(height: 12),
            Text(
              mime.isEmpty ? '未知类型' : mime,
              style: const TextStyle(color: AppTheme.ink, fontSize: 14),
            ),
            const SizedBox(height: 6),
            Text(
              '$size B · 当前版本暂不支持本地预览，可点右上角分享导出',
              style: const TextStyle(color: AppTheme.muted, fontSize: 12),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

class _ActionButtons extends StatelessWidget {
  const _ActionButtons({required this.content, required this.name});
  final ArtifactContent content;
  final String name;

  Future<void> _copy(BuildContext context) async {
    final t = content.text.isNotEmpty ? content.text : '(二进制内容，无法复制为文本)';
    await Clipboard.setData(ClipboardData(text: t));
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          backgroundColor: AppTheme.ink,
          content: Text('已复制到剪贴板', style: TextStyle(color: AppTheme.paper)),
          duration: Duration(seconds: 2),
        ),
      );
    }
  }

  Future<void> _share() async {
    // 先把字节写到临时文件，让 share_plus 走文件分享通道（更兼容各 App）
    final dir = await getTemporaryDirectory();
    final f = File('${dir.path}/$name');
    await f.writeAsBytes(content.bytes);
    await SharePlus.instance.share(
      ShareParams(
        files: [XFile(f.path, mimeType: content.mime)],
        subject: name,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        IconButton(
          tooltip: '复制全文',
          icon: const Icon(CupertinoIcons.doc_on_doc, color: AppTheme.paper, size: 18),
          onPressed: () => _copy(context),
        ),
        IconButton(
          tooltip: '分享 / 导出',
          icon: const Icon(CupertinoIcons.share, color: AppTheme.paper, size: 20),
          onPressed: _share,
        ),
      ],
    );
  }
}

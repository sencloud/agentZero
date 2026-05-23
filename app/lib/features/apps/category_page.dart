import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../providers/catalog.dart';
import '../../widgets/agent_row.dart';

class CategoryPage extends ConsumerWidget {
  const CategoryPage({super.key, required this.slug, required this.name});
  final String slug;
  final String name;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final list = ref.watch(agentListProvider(AgentListQuery(category: slug)));
    return Scaffold(
      appBar: AppBar(title: Text(name)),
      body: list.when(
        loading: () => const Center(child: CupertinoActivityIndicator()),
        error: (e, _) => Center(child: Text('加载失败：$e')),
        data: (items) => ListView.separated(
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
          itemBuilder: (_, i) => AgentRow(agent: items[i]),
          separatorBuilder: (_, _) => const Divider(),
          itemCount: items.length,
        ),
      ),
    );
  }
}

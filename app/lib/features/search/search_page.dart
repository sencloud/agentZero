import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/icon_mapper.dart';
import '../../providers/catalog.dart';
import '../../widgets/agent_row.dart';

class SearchPage extends ConsumerStatefulWidget {
  const SearchPage({super.key});
  @override
  ConsumerState<SearchPage> createState() => _SearchPageState();
}

class _SearchPageState extends ConsumerState<SearchPage> {
  final _controller = TextEditingController();
  String _query = '';
  Timer? _debounce;

  void _onChanged(String v) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), () {
      if (!mounted) return;
      setState(() => _query = v.trim());
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    _debounce?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final cats = ref.watch(categoriesProvider);

    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 8, 20, 0),
              child: Align(
                alignment: Alignment.centerLeft,
                child: Text('搜索', style: Theme.of(context).textTheme.displayLarge),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
              child: CupertinoSearchTextField(
                controller: _controller,
                placeholder: '搜索智能体、能力、开发者',
                onChanged: _onChanged,
                onSubmitted: (v) => setState(() => _query = v.trim()),
              ),
            ),
            Expanded(
              child: _query.isEmpty
                  ? cats.when(
                      loading: () => const Center(child: CupertinoActivityIndicator()),
                      error: (e, _) => Center(child: Text('$e')),
                      data: (items) => ListView(
                        padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
                        children: [
                          Text('热门分类', style: Theme.of(context).textTheme.headlineMedium),
                          const SizedBox(height: 12),
                          for (final c in items)
                            ListTile(
                              contentPadding: EdgeInsets.zero,
                              onTap: () => context.push('/category/${c.slug}?name=${Uri.encodeComponent(c.name)}'),
                              leading: Container(
                                width: 38,
                                height: 38,
                                decoration: BoxDecoration(
                                  color: CategoryIconSpec.colorFor(c.color).withValues(alpha: 0.15),
                                  borderRadius: BorderRadius.circular(10),
                                ),
                                alignment: Alignment.center,
                                child: Icon(CategoryIconSpec.iconFor(c.icon),
                                    color: CategoryIconSpec.colorFor(c.color), size: 22),
                              ),
                              title: Text(c.name, style: Theme.of(context).textTheme.titleMedium),
                              trailing: const Icon(CupertinoIcons.chevron_right, size: 14, color: Color(0xFF8E8E93)),
                            ),
                        ],
                      ),
                    )
                  : Consumer(
                      builder: (context, ref, _) {
                        final results = ref.watch(agentListProvider(AgentListQuery(query: _query)));
                        return results.when(
                          loading: () => const Center(child: CupertinoActivityIndicator()),
                          error: (e, _) => Center(child: Text('$e')),
                          data: (items) {
                            if (items.isEmpty) {
                              return Center(
                                child: Padding(
                                  padding: const EdgeInsets.all(32),
                                  child: Text('找不到与「$_query」相关的智能体',
                                      style: Theme.of(context).textTheme.bodyMedium),
                                ),
                              );
                            }
                            return ListView.separated(
                              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                              itemCount: items.length,
                              separatorBuilder: (_, _) => const Divider(),
                              itemBuilder: (_, i) => AgentRow(agent: items[i]),
                            );
                          },
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

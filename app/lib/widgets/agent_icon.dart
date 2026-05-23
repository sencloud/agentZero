import 'package:flutter/material.dart';

import '../core/icon_mapper.dart';

class AgentIcon extends StatelessWidget {
  const AgentIcon({super.key, required this.iconUrl, this.size = 60, this.radiusFactor = 0.226});

  final String iconUrl;
  final double size;
  final double radiusFactor;

  @override
  Widget build(BuildContext context) {
    final spec = AgentIconSpec.parse(iconUrl);
    final base = spec.color;
    final top = Color.lerp(Colors.white, base, 0.85)!;
    final bottom = base;
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(size * radiusFactor),
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [top, bottom],
        ),
        boxShadow: [
          BoxShadow(
            color: base.withValues(alpha: 0.25),
            blurRadius: size * 0.18,
            offset: Offset(0, size * 0.06),
          ),
        ],
      ),
      alignment: Alignment.center,
      child: Icon(
        spec.icon,
        color: Colors.white,
        size: size * 0.55,
      ),
    );
  }
}

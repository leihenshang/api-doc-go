// 分组树工具：后端以扁平数组返回某项目某类型的全部分组（含 p_id 父级），
// 这里在前端按 p_id 组装成树并做「带层级」的可见性扁平化，供 GroupTree 渲染。
import type { Group } from '@/types/entities';

/** 树节点：一个分组及其子分组。 */
export interface GroupNode {
    group: Group;
    children: GroupNode[];
}

/** 按 p_id 组装成森林；父级缺失（或 p_id=0）的作为根。入参已按 priority,id 有序，故同层保持原序。 */
export function buildGroupTree(groups: Group[]): GroupNode[] {
    const byId = new Map<number, GroupNode>();
    groups.forEach((g) => byId.set(g.id, { group: g, children: [] }));
    const roots: GroupNode[] = [];
    groups.forEach((g) => {
        const node = byId.get(g.id) as GroupNode;
        const parent = g.p_id ? byId.get(g.p_id) : undefined;
        if (parent) parent.children.push(node);
        else roots.push(node);
    });
    return roots;
}

/** 扁平可见行：level 从 1 起（顶级分组为 1），hasChildren 决定是否可展开。 */
export interface FlatGroup {
    id: number;
    title: string;
    level: number;
    hasChildren: boolean;
}

/** 深度优先扁平化，仅展开 isExpanded 为真的节点的子级。 */
export function flattenTree(roots: GroupNode[], isExpanded: (id: number) => boolean): FlatGroup[] {
    const out: FlatGroup[] = [];
    const walk = (nodes: GroupNode[], level: number): void => {
        nodes.forEach((n) => {
            const hasChildren = n.children.length > 0;
            out.push({ id: n.group.id, title: n.group.title, level, hasChildren });
            if (hasChildren && isExpanded(n.group.id)) walk(n.children, level + 1);
        });
    };
    walk(roots, 1);
    return out;
}

/** 收集所有「有子节点」的分组 id，用于默认全展开。 */
export function expandableIds(roots: GroupNode[]): number[] {
    const ids: number[] = [];
    const walk = (nodes: GroupNode[]): void => {
        nodes.forEach((n) => {
            if (n.children.length) {
                ids.push(n.group.id);
                walk(n.children);
            }
        });
    };
    walk(roots);
    return ids;
}

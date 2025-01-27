import { createQuery } from '@tanstack/svelte-query'
import { z } from 'zod'
import { branchTargetLocatorSchema, commitGraphLocatorSchema } from './locator';

// TODO: don't hardcode this
const bePort = 8080;
const beHost = `http://localhost:${bePort}`;

export const createPingQuery = () => {
    return createQuery({
        queryKey: ['ping'],
        queryFn: () => fetch(`${beHost}/api/ping`).then(res => res.text()),
    });
}
export type BranchName = string;
export type GoalID = string;
export type NodeID = string;

// @api /api/graph/branch-target-graph
export const goalBranchNodeSchema = z.object({
    parent_branch_target: branchTargetLocatorSchema,
    children_branch_targets: z.array(branchTargetLocatorSchema).optional(),
    commit_graph: commitGraphLocatorSchema,
    goal_name: z.string().optional(),
})
export type GoalBranchNode = z.infer<typeof goalBranchNodeSchema>;
// @api /api/graph/branch-target-graph
export const branchTargetGraphSchema = z.object({
    branch_targets: z.array(branchTargetLocatorSchema),
    subgraphs: z.array(goalBranchNodeSchema),
})
export type BranchTargetGraph = z.infer<typeof branchTargetGraphSchema>;

export const createBranchTargetGraphQuery = () => {
    return createQuery({
        queryKey: ['branch-target-graph'],
        queryFn: () => fetch(`${beHost}/api/graph/branch-target-graph`)
            .then(res => res.json())
            .then(data => branchTargetGraphSchema.parse(data)),
    });
}

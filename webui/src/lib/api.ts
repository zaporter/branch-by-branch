import { createQuery } from '@tanstack/svelte-query'
import { z } from 'zod'
import { branchTargetLocatorSchema, commitGraphLocatorSchema, nodeLocatorSchema, type CommitGraphLocator } from './locator';

// TODO: don't hardcode this
const bePort = 8080;
const beHost = `http://localhost:${bePort}`;
export type BranchName = string;
export type GoalID = string;
export type NodeID = string;

export const createPingQuery = () => {
    return createQuery({
        queryKey: ['ping'],
        queryFn: () => fetch(`${beHost}/api/ping`).then(res => res.text()),
    });
}

// @api /api/graph/branch-target-graph
export const goalBranchNodeSchema = z.object({
    parent_branch_target: branchTargetLocatorSchema,
    children_branch_targets: z.array(branchTargetLocatorSchema).optional(),
    commit_graph: commitGraphLocatorSchema,
    goal_name: z.string().optional(),
})
export type GoalBranchNode = z.infer<typeof goalBranchNodeSchema>;
// @api /api/graph/branch-target-graph-locators
export const branchTargetGraphLocatorsSchema = z.object({
    branch_targets: z.array(branchTargetLocatorSchema),
    subgraphs: z.array(goalBranchNodeSchema),
})
export type BranchTargetGraphLocators = z.infer<typeof branchTargetGraphLocatorsSchema>;

export const createBranchTargetGraphQuery = () => {
    return createQuery({
        queryKey: ['branch-target-graph'],
        queryFn: () => fetch(`${beHost}/api/graph/branch-target-graph-locators`)
            .then(res => res.json())
            .then(data => branchTargetGraphLocatorsSchema.parse(data)),
    });
}
export const nodeStateSchema = z.enum([
    'node_awaiting_goal_setup',
    'node_state_running_goal_setup',
    'node_awaiting_compilation',
    'node_state_running_compilation',
    'node_awaiting_inference',
    'node_state_running_inference',
    'node_state_done',
]);
export type NodeState = z.infer<typeof nodeStateSchema>;
export const nodeResultSchema = z.enum([
    'node_result_none',
    'node_result_success',
    'node_result_failure',
    'node_result_syntax_failure',
    'node_result_depth_exhaustion',
    'node_result_context_exhaustion',
]);
export type NodeResult = z.infer<typeof nodeResultSchema>;
export const graphStateSchema = z.enum([
    'graph_awaiting_goal_setup',
    'graph_in_progress',
    'graph_success',
    'graph_failed',
    'graph_goal_setup_failed',
]);
export type GraphState = z.infer<typeof graphStateSchema>;

//@api /api/graph/commit-graph-locators
export const commitGraphLocatorsNodeSchema = z.object({
    locator: nodeLocatorSchema,
    result: nodeResultSchema.optional(),
    state: nodeStateSchema,
    depth: z.number(),
    children: z.array(nodeLocatorSchema).default([]),
})
export type CommitGraphLocatorsNode = z.infer<typeof commitGraphLocatorsNodeSchema>;
//@api /api/graph/commit-graph-locators
export const commitGraphLocatorsSchema = z.object({
    state: graphStateSchema,
    root_node: nodeLocatorSchema,
    nodes: z.array(commitGraphLocatorsNodeSchema).default([]),
})
export type CommitGraphLocators = z.infer<typeof commitGraphLocatorsSchema>;

export const createCommitGraphQuery = (locator: CommitGraphLocator) => {
    return createQuery({
        queryKey: ['commit-graph', locator],
        queryFn: () => fetch(
            `${beHost}/api/graph/commit-graph-locators`,
            {
                method: 'POST',
                body: JSON.stringify(locator),
            }
        )
            .then(res => res.json())
            .then(data => commitGraphLocatorsSchema.parse(data)),
    });
}
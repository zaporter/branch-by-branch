import { createMutation, createQuery } from '@tanstack/svelte-query'
import { z } from 'zod'
import { branchTargetLocatorSchema, commitGraphLocatorSchema, nodeLocatorSchema, type BranchTargetLocator, type CommitGraphLocator, type NodeLocator } from './locator';

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
        refetchInterval: 1000,
        queryFn: () => fetch(`${beHost}/api/graph/branch-target-graph-locators`)
            .then(res => res.json())
            .then(data => branchTargetGraphLocatorsSchema.parse(data)),
    });
}
export const nodeMetadataSchema = z.object({
    was_manually_created: z.boolean().optional(),
    is_favorite: z.boolean().optional(),
    is_golden_sample: z.boolean().optional(),
    label: z.string().optional(),
}).strict() // ensure this is kept up to date with the backend

export type NodeMetadata = z.infer<typeof nodeMetadataSchema>;
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
    'node_result_terminated',
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
    metadata: nodeMetadataSchema,
    termination_requested: z.boolean(),
    children: z.array(nodeLocatorSchema).default([]),
    children_advantages: z.array(z.number()).default([]),
}).strict()
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
        refetchInterval: 1000,
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


//@api /api/graph/branch-target-stats
export const branchTargetStatsSchema = z.object({
    branch_name: z.string(),
    parent_branch_name: z.string().optional(),
    num_subgraphs: z.number(),
})
export type BranchTargetStats = z.infer<typeof branchTargetStatsSchema>;

export const createBranchTargetStatsQuery = (locator: BranchTargetLocator) => {
    return createQuery({
        queryKey: ['branch-target-stats', locator],
        refetchInterval: 1000,
        queryFn: () => fetch(`${beHost}/api/graph/branch-target-stats`, {
            method: 'POST',
            body: JSON.stringify(locator),
        })
            .then(res => res.json())
            .then(data => branchTargetStatsSchema.parse(data)),
    });
}

//@api /api/graph/commit-graph-stats
export const commitGraphStatsSchema = z.object({
    state: graphStateSchema,
    goal_id: z.string(),
})
export type CommitGraphStats = z.infer<typeof commitGraphStatsSchema>;

export const createCommitGraphStatsQuery = (locator: CommitGraphLocator) => {
    return createQuery({
        queryKey: ['commit-graph-stats', locator],
        refetchInterval: 1000,
        queryFn: () => fetch(`${beHost}/api/graph/commit-graph-stats`, {
            method: 'POST',
            body: JSON.stringify(locator),
        })
            .then(res => res.json())
            .then(data => commitGraphStatsSchema.parse(data)),
    });
}
const actionOutputSchema = z.object({
    action_name: z.string(),
    text: z.string(),
    exit_code: z.number(),
})
export type ActionOutput = z.infer<typeof actionOutputSchema>;
const compilationResultSchema = z.object({
    out: z.string(),
    exit_code: z.number(),
})
export type CompilationResult = z.infer<typeof compilationResultSchema>;

//@api /api/graph/node-stats
export const nodeStatsSchema = z.object({
    depth: z.number(),
    state: nodeStateSchema,
    result: nodeResultSchema,
    metadata: nodeMetadataSchema,
    termination_requested: z.boolean(),
    inference_output: z.string().optional(),
    action_outputs: z.array(actionOutputSchema).optional().nullable(),
    compilation_result: compilationResultSchema.optional().nullable(),
    prompt: z.string().optional(),
    branch_name: z.string(),
}).strict()
export type NodeStats = z.infer<typeof nodeStatsSchema>;

export const createNodeStatsQuery = (locator: NodeLocator) => {
    return createQuery({
        queryKey: ['node-stats', locator],
        refetchInterval: 1000,
        queryFn: () => fetch(`${beHost}/api/graph/node-stats`, {
            method: 'POST',
            body: JSON.stringify(locator),
        })
            .then(res => res.json())
            .then(data => nodeStatsSchema.parse(data)),
    });
}
export const createRequestNodeTerminationMutation = (locator: NodeLocator) => {
    return createMutation({
        mutationFn: () => fetch(`${beHost}/api/graph/request-node-termination`, {
            method: 'POST',
            body: JSON.stringify(locator),
        })
            .then(res => res.text()),
    });
}

export const createDeleteNodeMutation = (locator: NodeLocator) => {
    return createMutation({
        mutationFn: () => fetch(`${beHost}/api/graph/delete-node`, {
            method: 'POST',
            body: JSON.stringify(locator),
        })
            .then(res => res.text()),
    });
}

// @api /api/graph/set-commit-graph-state
export const setCommitGraphStateRequestSchema = z.object({
    commit_graph_locator: commitGraphLocatorSchema,
    state: graphStateSchema,
})
export type SetCommitGraphStateRequest = z.infer<typeof setCommitGraphStateRequestSchema>;

export const createSetCommitGraphStateMutation = () => {
    return createMutation({
        mutationFn: (request: SetCommitGraphStateRequest) => fetch(`${beHost}/api/graph/set-commit-graph-state`, {
            method: 'POST',
            body: JSON.stringify(request),
        })
            .then(res => res.text()),
    });
}

// @api /api/graph/create-node
export const createNodeRequestSchema = z.object({
    parent_node_locator: nodeLocatorSchema,
    inference_output: z.string(),
})
export type CreateNodeRequest = z.infer<typeof createNodeRequestSchema>;

// @api /api/graph/create-node
export const createNodeResponseSchema = z.object({
    node_locator: nodeLocatorSchema,
})
export type CreateNodeResponse = z.infer<typeof createNodeResponseSchema>;

export const createCreateNodeMutation = () => {
    return createMutation({
        mutationFn: (request: CreateNodeRequest) => fetch(`${beHost}/api/graph/create-node`, {
            method: 'POST',
            body: JSON.stringify(request),
        })
            .then(res => res.json())
            .then(data => createNodeResponseSchema.parse(data)),
    });
}

// @api /api/graph/set-node-metadata
export const setNodeMetadataRequestSchema = z.object({
    node_locator: nodeLocatorSchema,
    metadata: nodeMetadataSchema,
})
export type SetNodeMetadataRequest = z.infer<typeof setNodeMetadataRequestSchema>;

export const createSetNodeMetadataMutation = () => {
    return createMutation({
        mutationFn: (request: SetNodeMetadataRequest) => fetch(`${beHost}/api/graph/set-node-metadata`, {
            method: 'POST',
            body: JSON.stringify(request),
        })
            .then(res => res.text()),
    });
}

// @api /api/graph/save-golden-sample
export const saveGoldenSampleRequestSchema = z.object({
    node_locator: nodeLocatorSchema,
})
export type SaveGoldenSampleRequest = z.infer<typeof saveGoldenSampleRequestSchema>;

export const createSaveGoldenSampleMutation = () => {
    return createMutation({
        mutationFn: (request: SaveGoldenSampleRequest) => fetch(`${beHost}/api/graph/save-golden-sample`, {
            method: 'POST',
            body: JSON.stringify(request),
        })
            .then(res => res.json()),
    });
}
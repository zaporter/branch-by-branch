import { z } from "zod";

export const branchTargetLocatorSchema = z.object({
    branch_name: z.string(),
})
export type BranchTargetLocator = z.infer<typeof branchTargetLocatorSchema>;

export const commitGraphLocatorSchema = z.object({
    branch_target_locator: branchTargetLocatorSchema,
    goal_id: z.string(),
})
export type CommitGraphLocator = z.infer<typeof commitGraphLocatorSchema>;

export const nodeLocatorSchema = z.object({
    commit_graph_locator: commitGraphLocatorSchema,
    node_id: z.string(),
})
export type NodeLocator = z.infer<typeof nodeLocatorSchema>;

export const locatorToJSON = (locator: CommitGraphLocator | BranchTargetLocator | NodeLocator) => {
    return JSON.stringify(locator);
}

export type UnknownLocator = CommitGraphLocator | BranchTargetLocator | NodeLocator;

export const locatorFromString = (locator: string): UnknownLocator => {
    const parsed = JSON.parse(locator);
    return z.union([commitGraphLocatorSchema, branchTargetLocatorSchema, nodeLocatorSchema]).parse(parsed);
}

export const isBranchTargetLocator = (locator: UnknownLocator): locator is BranchTargetLocator => {
    return (
        'branch_name' in locator
    );
}
export const isCommitGraphLocator = (locator: UnknownLocator): locator is CommitGraphLocator => {
    return (
        'branch_target_locator' in locator
    );
}
export const isNodeLocator = (locator: UnknownLocator): locator is NodeLocator => {
    return (
        'node_id' in locator);
}
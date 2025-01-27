import { z } from "zod";

// Note to self:
// If I ever design nested locators again in my life,
// I should remember this file and design it as far away from this as possible.
// This is very pointlessly painful.

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

export const isBranchTargetLocator = (locator: UnknownLocator | undefined): locator is BranchTargetLocator => {
    return locator !== undefined && (
        'branch_name' in locator
    );
}
export const isCommitGraphLocator = (locator: UnknownLocator | undefined): locator is CommitGraphLocator => {
    return locator !== undefined && (
        'branch_target_locator' in locator
    );
}
export const isNodeLocator = (locator: UnknownLocator | undefined): locator is NodeLocator => {
    return locator !== undefined && (
        'node_id' in locator);
}

export const branchLocatorFromUnknown = (locator: UnknownLocator | undefined): BranchTargetLocator | undefined => {
    return isBranchTargetLocator(locator) ? locator : isCommitGraphLocator(locator) ? locator.branch_target_locator : isNodeLocator(locator) ? locator.commit_graph_locator.branch_target_locator : undefined;
}

export const commitGraphLocatorFromUnknown = (locator: UnknownLocator | undefined): CommitGraphLocator | undefined => {
    return isCommitGraphLocator(locator) ? locator : isNodeLocator(locator) ? locator.commit_graph_locator : undefined;
}

export const nodeLocatorFromUnknown = (locator: UnknownLocator | undefined): NodeLocator | undefined => {
    return isNodeLocator(locator) ? locator : undefined;
}
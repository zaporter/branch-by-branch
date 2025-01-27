import { redirect } from "@sveltejs/kit";
import type { PageData } from "./$types";

export const prerender = false;
export const ssr = false;

// @ts-ignore Something is broken
export const load = async ({ url }) => {
    if (url.pathname === '/') {
        redirect(302, '/graph');
    }
};
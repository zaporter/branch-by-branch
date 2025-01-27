import { redirect } from "@sveltejs/kit";

export const prerender = false;
export const ssr = false;

export const load = async ({ url }) => {
    if (url.pathname === '/') {
        redirect(302, '/graph');
    }
};
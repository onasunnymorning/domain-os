"use client";

import { useQuery } from "@tanstack/react-query";
import { getContactCount } from "@/lib/api/contacts";

export function useContactCount() {
    return useQuery({
        queryKey: ["contacts", "count"],
        queryFn: () => getContactCount(),
    });
}

"use client";

import { useQuery } from "@tanstack/react-query";
import { getHostCount } from "@/lib/api/hosts";

export function useHostCount() {
    return useQuery({
        queryKey: ["hosts", "count"],
        queryFn: () => getHostCount(),
    });
}

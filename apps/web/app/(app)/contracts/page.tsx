import { unstable_serialize, SWRConfig } from "swr";
import ContractsClient from "./ContractsClient";
import { getInitialContracts } from "@/lib/serverData";

export const revalidate = 60;

interface ContractsPageProps {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export default async function ContractsPage({
  searchParams,
}: ContractsPageProps) {
  const params = await searchParams;
  const sort = typeof params.sort === "string" ? params.sort : "added_at";
  const dir = params.dir === "asc" ? "asc" : "desc";
  const query = {
    cursor: undefined,
    limit: 20,
    network: undefined,
    tag: undefined,
    sort,
    dir,
  } as const;
  const initialContracts = await getInitialContracts(sort, dir);
  const fallback = initialContracts
    ? {
        [unstable_serialize(["contracts", query])]: initialContracts,
      }
    : {};

  return (
    <SWRConfig value={{ fallback }}>
      <ContractsClient initialSortColumn={sort} initialSortDirection={dir} />
    </SWRConfig>
  );
}

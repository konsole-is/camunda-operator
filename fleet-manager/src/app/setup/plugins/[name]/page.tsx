import { notFound } from "next/navigation";
import { RegistrationDetailLive } from "@/components/setup/plugins/registration-detail";
import { registrationTabOf } from "@/components/setup/plugins/registration-tabs";
import { loadRegistration } from "../load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function RegistrationPage({
  params,
  searchParams,
}: PageProps<"/setup/plugins/[name]">) {
  const { name } = await params;
  const { tab } = await searchParams;
  const initial = await loadRegistration(name);
  if (!initial) notFound();
  return (
    <RegistrationDetailLive
      initial={initial}
      tab={registrationTabOf(tab)}
      now={new Date().toISOString()}
    />
  );
}

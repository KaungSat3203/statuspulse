import { redirect } from "next/navigation";
import AuthForm from "@/components/auth/auth-form";
import { currentUser, serverAPI } from "@/lib/server-api";

export default async function Signup() {
  if (await currentUser(await serverAPI())) redirect("/");
  return <AuthForm mode="signup" />;
}

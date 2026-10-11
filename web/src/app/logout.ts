import { csrfHeaders, setCSRF } from "./transport";
import { setDataContext } from "./data-context";

// Only clear local identity state after the server confirms session revocation.
export async function endSession() {
  const response = await fetch("/auth/logout", {
    method: "POST",
    credentials: "same-origin",
    headers: csrfHeaders(),
  });
  if (!response.ok) throw new Error("request_failed");
  setCSRF("");
  setDataContext("");
}

/** True in the demo build, which runs the app against in-browser sample data. */
export const isDemo = import.meta.env.MODE === "demo";

/** The app URL for a team: /app?team=N normally, /demo/?team=N in the demo. */
export function teamURL(teamID: number) {
  return `${import.meta.env.BASE_URL}${isDemo ? "" : "app"}?team=${teamID}`;
}

import { useQuery } from "@tanstack/react-query";
import { api } from "../api";

export default function Logs() {
  const logs = useQuery({
    queryKey: ["logs"],
    queryFn: () => api.logs(150),
    refetchInterval: 5000,
  });

  return (
    <div>
      <h1>Logs</h1>
      <div className="panel">
        <p className="muted">In-process Librarian diagnostic ring (not durable across restart).</p>
        {logs.isError && <div className="error-box">{(logs.error as Error).message}</div>}
        <table>
          <thead>
            <tr>
              <th>At</th>
              <th>Level</th>
              <th>Message</th>
            </tr>
          </thead>
          <tbody>
            {(logs.data?.lines || [])
              .slice()
              .reverse()
              .map((line, i) => (
                <tr key={i}>
                  <td className="mono">{line.at}</td>
                  <td>{line.level}</td>
                  <td>{line.message}</td>
                </tr>
              ))}
          </tbody>
        </table>
        {!logs.data?.lines?.length && <p className="muted">No lines yet.</p>}
      </div>
    </div>
  );
}

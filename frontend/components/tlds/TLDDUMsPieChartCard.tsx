import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from "recharts";

interface Props {
  data: { name: string; value: number; clid: string }[];
  onClick?: () => void;
}

// A vibrant, colorful palette
const COLORS = [
  '#0088FE', '#00C49F', '#FFBB28', '#FF8042', 
  '#8884d8', '#8dd1e1', '#a4de6c', '#d0ed57',
  '#ffc658', '#4da2ff', '#fb7a8f', '#e592f6'
];

const CustomTooltip = ({ active, payload }: any) => {
  if (active && payload && payload.length) {
    const data = payload[0].payload;
    let label = data.clid;
    if (data.clid === "other") label = "Other";
    return (
      <div className="bg-popover text-popover-foreground px-3 py-2 rounded-md shadow-md text-sm border font-medium">
        {label} ({Number(data.value).toLocaleString()})
      </div>
    );
  }
  return null;
};

export function TLDDUMsPieChartCard({ data, onClick }: Props) {
  // Filter out registrars with 0 domains if any
  const chartData = data.filter(d => d.value > 0);

  if (chartData.length === 0) {
    return (
      <Card onClick={onClick} className={onClick ? "cursor-pointer hover:bg-muted/50 transition-colors" : ""}>
        <CardContent className="flex items-center gap-4 p-6">
          <div className="h-16 w-16 shrink-0 bg-muted rounded-full flex items-center justify-center text-xs text-muted-foreground">
            0%
          </div>
          <div>
            <p className="text-sm font-medium text-muted-foreground">DUMs Split</p>
            <p className="text-xs text-muted-foreground">No data available</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  // Aggregate anything beyond the Top 10 into "Other"
  let processedData = chartData.slice(0, 10);
  let otherValue = chartData.slice(10).reduce((acc, curr) => acc + curr.value, 0);

  if (otherValue > 0) {
    processedData.push({ name: "Other", clid: "other", value: otherValue });
  }

  return (
    <Card onClick={onClick} className={onClick ? "cursor-pointer hover:bg-muted/50 transition-colors" : ""}>
      <CardContent className="flex items-center gap-4 p-6">
        <div className="h-16 w-16 shrink-0">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={processedData}
                cx="50%"
                cy="50%"
                innerRadius={0}
                outerRadius={32}
                dataKey="value"
                nameKey="name"
                stroke="none"
              >
                {processedData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip content={<CustomTooltip />} />
            </PieChart>
          </ResponsiveContainer>
        </div>
        <div>
          <p className="text-sm font-medium text-muted-foreground">DUMs Share</p>
          <p className="text-xs text-muted-foreground">Top {Math.min(chartData.length, 10)} Registrars</p>
        </div>
      </CardContent>
    </Card>
  );
}

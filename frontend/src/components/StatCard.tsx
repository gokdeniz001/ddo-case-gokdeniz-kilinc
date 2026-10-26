import React from 'react';

interface StatCardProps {
  label: string;
  value: string;
  meta: string;
  valueColor?: string;
  metaColor?: string;
}

export const StatCard: React.FC<StatCardProps> = ({
  label,
  value,
  meta,
  valueColor,
  metaColor,
}) => {
  return (
    <div className="glass-card stat-card">
      <div className="stat-label">{label}</div>
      <div className="stat-value" style={valueColor ? { color: valueColor } : undefined}>
        {value}
      </div>
      <div className="stat-meta" style={metaColor ? { color: metaColor } : undefined}>
        {meta}
      </div>
    </div>
  );
};

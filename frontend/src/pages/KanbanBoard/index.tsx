import React from 'react';
import { Spin, message } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { DndContext, useDroppable, useDraggable, useSensor, useSensors, PointerSensor } from '@dnd-kit/core';
import type { DragEndEvent } from '@dnd-kit/core';
import { listIssues, transitionIssueStatus } from '../../api/issue';
import { useNavigate, useParams } from 'react-router-dom';
import IssueCard from '../../components/IssueCard';
import { ISSUE_STATUS_LABELS, ALLOWED_TRANSITIONS } from '../../types/common';
import type { Issue } from '../../types/issue';
import type { IssueStatus } from '../../types/common';

const KANBAN_STATUSES: IssueStatus[] = ['todo', 'in_progress', 'review', 'testing', 'done'];

interface Props {
  projectId?: string;
}

function DroppableColumn({ status, children }: { status: string; children: React.ReactNode }) {
  const { setNodeRef, isOver } = useDroppable({ id: status });
  return (
    <div
      ref={setNodeRef}
      style={{
        flex: 1,
        minWidth: 220,
        background: isOver ? '#e6f7ff' : '#fafafa',
        borderRadius: 8,
        padding: 12,
        minHeight: 400,
      }}
    >
      <h4 style={{ textAlign: 'center', marginBottom: 12 }}>{ISSUE_STATUS_LABELS[status as IssueStatus]}</h4>
      {children}
    </div>
  );
}

function DraggableCard({ issue }: { issue: Issue }) {
  const navigate = useNavigate();
  const { attributes, listeners, setNodeRef, transform } = useDraggable({ id: issue.id, data: { issue } });
  const style = transform ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` } : undefined;

  return (
    <div ref={setNodeRef} style={style} {...listeners} {...attributes}>
      <IssueCard issue={issue} onClick={() => navigate(`/issues/${issue.id}`)} />
    </div>
  );
}

const KanbanBoard: React.FC<Props> = ({ projectId: propProjectId }) => {
  const params = useParams<{ projectId: string }>();
  const projectId = propProjectId || params.projectId!;
  const queryClient = useQueryClient();

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    })
  );

  const { data, isLoading } = useQuery({
    queryKey: ['issues', projectId, { page_size: 200 }],
    queryFn: () => listIssues(projectId, { page: 1, page_size: 200 }),
    enabled: !!projectId,
  });

  const transitionMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) => transitionIssueStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
    },
    onError: () => {
      message.error('状态流转失败');
    },
  });

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over) return;
    const issue = active.data.current?.issue as Issue;
    const newStatus = over.id as string;
    if (issue.status === newStatus) return;

    const allowed = ALLOWED_TRANSITIONS[issue.status as IssueStatus] || [];
    if (!allowed.includes(newStatus as IssueStatus)) {
      message.warning('不允许该状态流转');
      return;
    }
    transitionMutation.mutate({ id: issue.id, status: newStatus });
  };

  if (isLoading) return <Spin />;

  const issues = data?.data || [];

  return (
    <DndContext sensors={sensors} onDragEnd={handleDragEnd}>
      <div style={{ display: 'flex', gap: 12, overflowX: 'auto' }}>
        {KANBAN_STATUSES.map((status) => (
          <DroppableColumn key={status} status={status}>
            {issues
              .filter((i) => i.status === status)
              .map((issue) => (
                <DraggableCard key={issue.id} issue={issue} />
              ))}
          </DroppableColumn>
        ))}
      </div>
    </DndContext>
  );
};

export default KanbanBoard;

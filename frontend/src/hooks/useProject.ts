import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getProject } from '../api/project';

export function useCurrentProject() {
  const { projectId } = useParams<{ projectId: string }>();

  const { data: project, isLoading } = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => getProject(projectId!),
    enabled: !!projectId,
  });

  return { project, projectId: projectId!, isLoading };
}

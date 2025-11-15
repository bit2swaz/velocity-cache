'use server'

import { revalidatePath } from 'next/cache'

import { auth } from '@clerk/nextjs/server'

import { prisma } from '@/lib/prisma'

export async function createProject(formData: FormData) {
  const { userId } = await auth()
  if (!userId) {
    return { success: false, error: 'Not authenticated.' }
  }

  const nameValue = formData.get('name')
  const name = typeof nameValue === 'string' ? nameValue.trim() : ''
  if (!name) {
    return { success: false, error: 'Project name is required.' }
  }

  const orgMember = await prisma.orgMember.findFirst({
    where: { userId },
    select: { orgId: true },
  })

  if (!orgMember?.orgId) {
    return { success: false, error: 'No organization found for this user.' }
  }

  const project = await prisma.project.create({
    data: {
      name,
      orgId: orgMember.orgId,
    },
  })

  revalidatePath('/projects')

  return { success: true, projectId: project.id }
}

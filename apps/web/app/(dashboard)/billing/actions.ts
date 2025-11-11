'use server'

import { revalidatePath } from 'next/cache'

import { auth } from '@clerk/nextjs/server'

import { prisma } from '@/lib/prisma'

export async function joinWaitlist() {
  const { userId } = await auth()
  if (!userId) {
    return { success: false }
  }

  const orgMember = await prisma.orgMember.findFirst({ where: { userId } })
  if (!orgMember) {
    return { success: false }
  }

  try {
    await prisma.waitlist.create({
      data: {
        userId,
        orgId: orgMember.orgId,
      },
    })
  } catch {
    return { success: false }
  }

  revalidatePath('/billing')
  return { success: true }
}

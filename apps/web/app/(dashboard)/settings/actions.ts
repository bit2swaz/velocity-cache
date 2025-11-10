'use server';

import crypto from 'crypto';

import { auth } from '@clerk/nextjs/server';
import { revalidatePath } from 'next/cache';

import { prisma } from '@/lib/prisma';

export async function generateApiToken(note: string) {
  const { userId } = await auth();

  if (!userId) {
    throw new Error('Unauthorized');
  }

  const token = `vc_${crypto.randomBytes(16).toString('hex')}`;
  const hash = crypto.createHash('sha256').update(token).digest('hex');

  await prisma.apiToken.create({
    data: {
      userId,
      tokenHash: hash,
      note,
    },
  });

  revalidatePath('/settings');

  return token;
}

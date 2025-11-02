// apps/web/src/app/api/webhooks/clerk/route.ts
import { Webhook } from 'svix';
import { headers } from 'next/headers';
import { WebhookEvent } from '@clerk/nextjs/server';
import { PrismaClient } from '@prisma/client';

const prisma = new PrismaClient();

export async function POST(req: Request) {
  const WEBHOOK_SECRET = process.env.CLERK_WEBHOOK_SECRET;
  if (!WEBHOOK_SECRET) {
    throw new Error('Please add CLERK_WEBHOOK_SECRET from Clerk Dashboard to .env');
  }

  const headerPayload = await headers();
  const svix_id = headerPayload.get("svix-id");
  const svix_timestamp = headerPayload.get("svix-timestamp");
  const svix_signature = headerPayload.get("svix-signature");

  if (!svix_id || !svix_timestamp || !svix_signature) {
    return new Response('Error: No svix headers', { status: 400 });
  }

  const payload = await req.json();
  const body = JSON.stringify(payload);
  const wh = new Webhook(WEBHOOK_SECRET);
  let evt: WebhookEvent;

  try {
    evt = wh.verify(body, {
      "svix-id": svix_id,
      "svix-timestamp": svix_timestamp,
      "svix-signature": svix_signature,
    }) as WebhookEvent;
  } catch (err) {
    console.error('Error verifying webhook:', err);
    return new Response('Error verifying webhook', { status: 400 });
  }

  const { id } = evt.data;
  const eventType = evt.type;

  // --- SYNC LOGIC ---
  if (eventType === 'user.created' || eventType === 'user.updated') {
    const { id, email_addresses, first_name, last_name } = evt.data;
    const email = email_addresses[0]?.email_address;
    if (!email) {
      return new Response('Error: No email address found', { status: 400 });
    }

    await prisma.user.upsert({
      where: { id: id },
      update: {
        email: email,
        name: `${first_name} ${last_name}`,
      },
      create: {
        id: id,
        email: email,
        name: `${first_name} ${last_name}`,
      },
    });
  }

  if (eventType === 'user.deleted') {
    // Note: You might want to cascade delete or handle this differently
    await prisma.user.delete({
      where: { id: id },
    });
  }
  // --- END SYNC LOGIC ---

  return new Response('', { status: 200 });
}
import type { ComponentProps } from "react";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

export function AlertDialog(props: ComponentProps<typeof Dialog>) { return <Dialog {...props} />; }
export function AlertDialogContent(props: ComponentProps<typeof DialogContent>) { return <DialogContent showCloseButton={false} className="max-w-md" {...props} />; }
export function AlertDialogHeader(props: ComponentProps<typeof DialogHeader>) { return <DialogHeader {...props} />; }
export function AlertDialogFooter(props: ComponentProps<typeof DialogFooter>) { return <DialogFooter className="sm:justify-end" {...props} />; }
export function AlertDialogTitle(props: ComponentProps<typeof DialogTitle>) { return <DialogTitle {...props} />; }
export function AlertDialogDescription(props: ComponentProps<typeof DialogDescription>) { return <DialogDescription {...props} />; }
export function AlertDialogAction(props: ComponentProps<typeof Button>) { return <Button {...props} />; }
export function AlertDialogCancel(props: ComponentProps<typeof Button>) { return <Button variant="outline" {...props} />; }
